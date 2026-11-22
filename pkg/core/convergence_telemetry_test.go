package core

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConvergenceTelemetryEngine_CleanCycleAndConvergence(t *testing.T) {
	engine := NewConvergenceTelemetryEngine(50)
	urn := "urn:svc:k8s:ingress-controller"

	// 1. Record healthy cycle
	err := engine.RecordCycle(ConvergenceCycleSample{
		URN:                  urn,
		DurationMs:           45.0,
		IterationsToConverge: 1,
		Success:              true,
		DriftDeltaCount:      0,
		Timestamp:            time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	diag, err := engine.GetResourceDiagnostic(urn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diag.HealthState != ConvergenceStateConverged {
		t.Errorf("expected CONVERGED state, got: %s", diag.HealthState)
	}
	if diag.SuccessRate != 1.0 {
		t.Errorf("expected 1.0 success rate, got: %f", diag.SuccessRate)
	}
	if diag.AvgDurationMs != 45.0 {
		t.Errorf("expected 45.0ms avg duration, got: %f", diag.AvgDurationMs)
	}
}

func TestConvergenceTelemetryEngine_DivergenceDetection(t *testing.T) {
	engine := NewConvergenceTelemetryEngine(50)
	urn := "urn:svc:k8s:database"

	// Cycle 1: drift = 1
	_ = engine.RecordCycle(ConvergenceCycleSample{
		URN:                  urn,
		DurationMs:           50.0,
		IterationsToConverge: 1,
		Success:              true,
		DriftDeltaCount:      1,
	})

	// Cycle 2: drift increased to 5 -> DIVERGING
	_ = engine.RecordCycle(ConvergenceCycleSample{
		URN:                  urn,
		DurationMs:           65.0,
		IterationsToConverge: 2,
		Success:              true,
		DriftDeltaCount:      5,
	})

	diag, _ := engine.GetResourceDiagnostic(urn)
	if diag.HealthState != ConvergenceStateDiverging {
		t.Fatalf("expected DIVERGING state when drift expands, got: %s", diag.HealthState)
	}
	if len(diag.Advisories) == 0 {
		t.Fatal("expected advisory for diverging state")
	}
}

func TestConvergenceTelemetryEngine_StalledDetection(t *testing.T) {
	engine := NewConvergenceTelemetryEngine(50)
	urn := "urn:svc:k8s:failing-job"

	// Record failed passes
	for i := 0; i < 4; i++ {
		_ = engine.RecordCycle(ConvergenceCycleSample{
			URN:                  urn,
			DurationMs:           120.0,
			IterationsToConverge: 8,
			Success:              false,
			DriftDeltaCount:      2,
		})
	}

	diag, _ := engine.GetResourceDiagnostic(urn)
	if diag.HealthState != ConvergenceStateStalled {
		t.Fatalf("expected STALLED state, got: %s", diag.HealthState)
	}
}

func TestConvergenceTelemetryEngine_GlobalDiagnosticAndPrometheus(t *testing.T) {
	engine := NewConvergenceTelemetryEngine(50)

	_ = engine.RecordCycle(ConvergenceCycleSample{
		URN:                  "urn:1",
		DurationMs:           10.0,
		IterationsToConverge: 1,
		Success:              true,
	})
	_ = engine.RecordCycle(ConvergenceCycleSample{
		URN:                  "urn:2",
		DurationMs:           50.0,
		IterationsToConverge: 1,
		Success:              true,
	})

	global := engine.GetGlobalDiagnostic()
	if global.TotalTrackedResources != 2 {
		t.Fatalf("expected 2 tracked resources, got %d", global.TotalTrackedResources)
	}
	if global.OverallSuccessRate != 1.0 {
		t.Errorf("expected 1.0 overall success rate, got %f", global.OverallSuccessRate)
	}

	prom := engine.ExportPrometheusMetrics()
	if !strings.Contains(prom, "reconciler_convergence_cycles_total 2") {
		t.Fatalf("missing total cycles in prom output:\n%s", prom)
	}
	if !strings.Contains(prom, `reconciler_resources_by_state{state="healthy"} 2`) {
		t.Fatalf("missing healthy resources in prom output:\n%s", prom)
	}
}

func TestConvergenceTelemetryEngine_Concurrency(t *testing.T) {
	engine := NewConvergenceTelemetryEngine(100)

	var wg sync.WaitGroup
	workers := 10
	iterations := 20

	for i := 0; i < workers; i++ {
		wg.Add(1)
		wID := i
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				urn := "urn:worker:" + string(rune('a'+wID))
				_ = engine.RecordCycle(ConvergenceCycleSample{
					URN:                  urn,
					DurationMs:           float64(j * 10),
					IterationsToConverge: 1,
					Success:              true,
				})
				_, _ = engine.GetResourceDiagnostic(urn)
				_ = engine.GetGlobalDiagnostic()
			}
		}()
	}

	wg.Wait()
	global := engine.GetGlobalDiagnostic()
	if global.TotalCyclesRecorded != workers*iterations {
		t.Fatalf("expected %d total cycles, got %d", workers*iterations, global.TotalCyclesRecorded)
	}
}
