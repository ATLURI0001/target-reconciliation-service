package core

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReconcilerTelemetry_Initialization(t *testing.T) {
	cfg := DefaultTelemetryConfig()
	col := NewReconcilerTelemetryCollector(cfg)

	snap := col.Snapshot()
	if snap.TotalRuns != 0 {
		t.Errorf("expected 0 total runs, got %d", snap.TotalRuns)
	}
	if snap.Health != HealthStateHealthy {
		t.Errorf("expected HEALTHY state, got %s", snap.Health)
	}
	if snap.RollingErrorRate != 0.0 {
		t.Errorf("expected 0.0 error rate, got %f", snap.RollingErrorRate)
	}
}

func TestReconcilerTelemetry_ExecutionTracking(t *testing.T) {
	cfg := TelemetryConfig{
		WindowSize:                  10,
		DegradedErrorThreshold:      0.30,
		UnhealthyErrorThreshold:     0.60,
		DegradedConsecutiveFailures: 3,
		UnhealthyConsecutiveFailures: 5,
		StaleThreshold:              time.Hour,
	}
	col := NewReconcilerTelemetryCollector(cfg)

	col.RecordExecution(true, 50*time.Millisecond, false, false)
	col.RecordExecution(true, 70*time.Millisecond, true, false)
	col.RecordExecution(false, 120*time.Millisecond, true, true)

	snap := col.Snapshot()
	if snap.TotalRuns != 3 {
		t.Fatalf("expected 3 total runs, got %d", snap.TotalRuns)
	}
	if snap.SuccessCount != 2 {
		t.Errorf("expected 2 successes, got %d", snap.SuccessCount)
	}
	if snap.FailureCount != 1 {
		t.Errorf("expected 1 failure, got %d", snap.FailureCount)
	}
	if snap.DriftDetectedCount != 2 {
		t.Errorf("expected 2 drifts, got %d", snap.DriftDetectedCount)
	}
	if snap.RollbackCount != 1 {
		t.Errorf("expected 1 rollback, got %d", snap.RollbackCount)
	}
	if snap.MaxDuration != 120*time.Millisecond {
		t.Errorf("expected max duration 120ms, got %v", snap.MaxDuration)
	}
	if snap.ConsecutiveFailures != 1 {
		t.Errorf("expected 1 consecutive failure, got %d", snap.ConsecutiveFailures)
	}

	// 1 failure out of 3 runs = 33.33% error rate -> Degraded (threshold 30%)
	if snap.Health != HealthStateDegraded {
		t.Errorf("expected DEGRADED health, got %s (reason: %s)", snap.Health, snap.HealthReason)
	}
}

func TestReconcilerTelemetry_ConsecutiveFailuresTrip(t *testing.T) {
	cfg := TelemetryConfig{
		WindowSize:                  20,
		DegradedErrorThreshold:      0.80, // High so consecutive failure trips first
		UnhealthyErrorThreshold:     0.90,
		DegradedConsecutiveFailures: 3,
		UnhealthyConsecutiveFailures: 5,
		StaleThreshold:              time.Hour,
	}
	col := NewReconcilerTelemetryCollector(cfg)

	// 10 initial successes
	for i := 0; i < 10; i++ {
		col.RecordExecution(true, 10*time.Millisecond, false, false)
	}
	if col.Snapshot().Health != HealthStateHealthy {
		t.Errorf("expected HEALTHY state")
	}

	// 3 consecutive failures -> Degraded
	for i := 0; i < 3; i++ {
		col.RecordExecution(false, 10*time.Millisecond, false, false)
	}
	snap := col.Snapshot()
	if snap.Health != HealthStateDegraded {
		t.Errorf("expected DEGRADED state after 3 consecutive failures, got %s", snap.Health)
	}

	// 2 more failures (total 5 consecutive) -> Unhealthy
	col.RecordExecution(false, 10*time.Millisecond, false, false)
	col.RecordExecution(false, 10*time.Millisecond, false, false)
	snap = col.Snapshot()
	if snap.Health != HealthStateUnhealthy {
		t.Errorf("expected UNHEALTHY state after 5 consecutive failures, got %s", snap.Health)
	}

	// 1 success resets consecutive failures
	col.RecordExecution(true, 10*time.Millisecond, false, false)
	snap = col.Snapshot()
	if snap.ConsecutiveFailures != 0 {
		t.Errorf("expected 0 consecutive failures after recovery, got %d", snap.ConsecutiveFailures)
	}
}

func TestReconcilerTelemetry_RollingWindowWrapping(t *testing.T) {
	cfg := TelemetryConfig{
		WindowSize:                  5,
		DegradedErrorThreshold:      0.40,
		UnhealthyErrorThreshold:     0.70,
		DegradedConsecutiveFailures: 10,
		UnhealthyConsecutiveFailures: 10,
		StaleThreshold:              time.Hour,
	}
	col := NewReconcilerTelemetryCollector(cfg)

	// 5 failures
	for i := 0; i < 5; i++ {
		col.RecordExecution(false, 10*time.Millisecond, false, false)
	}
	if col.Snapshot().RollingErrorRate != 1.0 {
		t.Errorf("expected 1.0 error rate, got %f", col.Snapshot().RollingErrorRate)
	}

	// Overwrite with 5 successes
	for i := 0; i < 5; i++ {
		col.RecordExecution(true, 10*time.Millisecond, false, false)
	}
	if col.Snapshot().RollingErrorRate != 0.0 {
		t.Errorf("expected 0.0 error rate after window overwritten, got %f", col.Snapshot().RollingErrorRate)
	}
}

func TestReconcilerTelemetry_PrometheusExport(t *testing.T) {
	col := NewReconcilerTelemetryCollector(DefaultTelemetryConfig())
	col.RecordExecution(true, 25*time.Millisecond, true, false)

	prom := col.ExportPrometheusMetrics()
	if !strings.Contains(prom, "reconciler_total_runs_total 1") {
		t.Errorf("expected reconciler_total_runs_total 1 in export:\n%s", prom)
	}
	if !strings.Contains(prom, "reconciler_success_total 1") {
		t.Errorf("expected reconciler_success_total 1 in export")
	}
	if !strings.Contains(prom, "reconciler_drift_detected_total 1") {
		t.Errorf("expected reconciler_drift_detected_total 1 in export")
	}
	if !strings.Contains(prom, "reconciler_health_status 1") {
		t.Errorf("expected healthy status code 1 in export")
	}
}

func TestReconcilerTelemetry_ConcurrentSafety(t *testing.T) {
	col := NewReconcilerTelemetryCollector(DefaultTelemetryConfig())
	var wg sync.WaitGroup
	workers := 8
	iterations := 100

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				success := (workerID+i)%5 != 0
				col.RecordExecution(success, 5*time.Millisecond, i%2 == 0, false)
				_ = col.Snapshot()
			}
		}(w)
	}

	wg.Wait()
	snap := col.Snapshot()
	expectedRuns := uint64(workers * iterations)
	if snap.TotalRuns != expectedRuns {
		t.Errorf("expected %d total runs, got %d", expectedRuns, snap.TotalRuns)
	}
}
