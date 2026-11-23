package core

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestDriftTelemetryProfiler_EmptyState(t *testing.T) {
	profiler := NewDriftTelemetryProfiler(DefaultDriftTelemetryConfig())
	diag := profiler.Diagnose()

	if diag.TotalEvaluated != 0 || diag.DriftedCount != 0 || diag.Status != DriftHealthy {
		t.Fatalf("unexpected empty diagnostic: %+v", diag)
	}
	if len(diag.Recommendations) != 0 || len(diag.FlappingResources) != 0 {
		t.Fatalf("expected empty recommendations and flapping list")
	}
}

func TestDriftTelemetryProfiler_InputValidation(t *testing.T) {
	profiler := NewDriftTelemetryProfiler(DefaultDriftTelemetryConfig())

	// Empty resource ID
	err := profiler.Record(DriftEvent{ResourceID: ""})
	if err != ErrEmptyResourceID {
		t.Fatalf("expected ErrEmptyResourceID, got: %v", err)
	}

	// Negative latency
	err = profiler.Record(DriftEvent{ResourceID: "res-1", ResolutionLatencyMs: -10.5})
	if err != ErrNegativeLatency {
		t.Fatalf("expected ErrNegativeLatency, got: %v", err)
	}

	// Valid record
	err = profiler.Record(DriftEvent{ResourceID: "res-1", ResolutionLatencyMs: 15.0})
	if err != nil {
		t.Fatalf("expected valid record to pass, got: %v", err)
	}
}

func TestDriftTelemetryProfiler_HealthyAndElevatedDrift(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	cfg := DefaultDriftTelemetryConfig()
	cfg.NowFunc = func() time.Time { return now }

	profiler := NewDriftTelemetryProfiler(cfg)

	// 10 events, 1 drifted (10% drift rate) -> Healthy
	for i := 0; i < 9; i++ {
		_ = profiler.Record(DriftEvent{ResourceID: fmt.Sprintf("res-%d", i), HasDrift: false, Timestamp: now})
	}
	_ = profiler.Record(DriftEvent{ResourceID: "res-drift-1", HasDrift: true, ResolutionLatencyMs: 50.0, Timestamp: now})

	diag := profiler.Diagnose()
	if diag.Status != DriftHealthy || diag.DriftRate != 0.10 {
		t.Fatalf("expected DriftHealthy with 0.10 rate, got %s (rate %f)", diag.Status, diag.DriftRate)
	}

	// Add 3 more drifted events: now 4/13 = ~30.7% drift rate -> Warning Elevated
	for i := 0; i < 3; i++ {
		_ = profiler.Record(DriftEvent{ResourceID: fmt.Sprintf("extra-drift-%d", i), HasDrift: true, ResolutionLatencyMs: 60.0, Timestamp: now})
	}

	diag = profiler.Diagnose()
	if diag.Status != DriftWarningElevated {
		t.Fatalf("expected DriftWarningElevated, got %s (rate %f)", diag.Status, diag.DriftRate)
	}
	if len(diag.Recommendations) == 0 {
		t.Fatalf("expected recommendations for elevated drift")
	}
}

func TestDriftTelemetryProfiler_FlappingDetection(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	cfg := DefaultDriftTelemetryConfig()
	cfg.FlapThreshold = 3
	cfg.FlapWindow = 5 * time.Minute
	cfg.NowFunc = func() time.Time { return now }

	profiler := NewDriftTelemetryProfiler(cfg)

	// Ingest 20 non-drift events across resources
	for i := 0; i < 20; i++ {
		_ = profiler.Record(DriftEvent{ResourceID: fmt.Sprintf("stable-res-%d", i), HasDrift: false, Timestamp: now})
	}

	// Flap res-flapper 3 times within 2 minutes
	_ = profiler.Record(DriftEvent{ResourceID: "res-flapper", HasDrift: true, Timestamp: now.Add(-2 * time.Minute)})
	_ = profiler.Record(DriftEvent{ResourceID: "res-flapper", HasDrift: true, Timestamp: now.Add(-1 * time.Minute)})
	_ = profiler.Record(DriftEvent{ResourceID: "res-flapper", HasDrift: true, Timestamp: now})

	diag := profiler.Diagnose()
	if diag.Status != DriftWarningFlapping {
		t.Fatalf("expected DriftWarningFlapping, got %s", diag.Status)
	}
	if len(diag.FlappingResources) != 1 || diag.FlappingResources[0] != "res-flapper" {
		t.Fatalf("expected res-flapper in flapping list, got %v", diag.FlappingResources)
	}

	// Now advance time 6 minutes past window
	now = now.Add(6 * time.Minute)
	diagAfterWindow := profiler.Diagnose()
	if len(diagAfterWindow.FlappingResources) != 0 {
		t.Fatalf("expected flapping list to be pruned after flap window, got %v", diagAfterWindow.FlappingResources)
	}
}

func TestDriftTelemetryProfiler_CriticalControllerFight(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	cfg := DefaultDriftTelemetryConfig()
	cfg.CriticalDriftRateThreshold = 0.50
	cfg.NowFunc = func() time.Time { return now }

	profiler := NewDriftTelemetryProfiler(cfg)

	// 10 events, 6 drifted (60% drift rate)
	for i := 0; i < 4; i++ {
		_ = profiler.Record(DriftEvent{ResourceID: fmt.Sprintf("res-ok-%d", i), HasDrift: false, Timestamp: now})
	}
	for i := 0; i < 6; i++ {
		_ = profiler.Record(DriftEvent{ResourceID: fmt.Sprintf("res-drift-%d", i), HasDrift: true, Timestamp: now})
	}

	diag := profiler.Diagnose()
	if diag.Status != DriftCriticalControllerFight {
		t.Fatalf("expected DriftCriticalControllerFight, got %s", diag.Status)
	}
}

func TestDriftTelemetryProfiler_ConcurrencyAndReset(t *testing.T) {
	profiler := NewDriftTelemetryProfiler(DefaultDriftTelemetryConfig())
	var wg sync.WaitGroup

	for w := 0; w < 10; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < 30; i++ {
				_ = profiler.Record(DriftEvent{
					ResourceID:          fmt.Sprintf("w%d-res-%d", workerID, i%5),
					HasDrift:            i%2 == 0,
					ResolutionLatencyMs: float64(10 + i),
				})
			}
		}(w)
	}

	wg.Wait()
	diag := profiler.Diagnose()
	if diag.TotalEvaluated != 300 {
		t.Fatalf("expected 300 total evaluated, got %d", diag.TotalEvaluated)
	}

	profiler.Reset()
	diagReset := profiler.Diagnose()
	if diagReset.TotalEvaluated != 0 {
		t.Fatalf("expected 0 evaluated after Reset(), got %d", diagReset.TotalEvaluated)
	}
}
