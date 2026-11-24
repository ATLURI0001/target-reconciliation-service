package core

import (
	"strings"
	"testing"
	"time"
)

func TestBatchTelemetryEngine_NominalRecording(t *testing.T) {
	cfg := DefaultBatchTelemetryConfig()
	engine := NewBatchTelemetryEngine(cfg)

	// 5 nominal batches
	for i := 1; i <= 5; i++ {
		engine.RecordBatch(BatchReconcileEvent{
			BatchID:          "b1",
			TargetsProcessed: 100,
			CreatedCount:     10,
			UpdatedCount:     20,
			DeletedCount:     5,
			UnchangedCount:   65,
			Duration:         time.Duration(i*100) * time.Millisecond,
			Success:          true,
		})
	}

	snap := engine.Snapshot()
	if snap.TotalBatches != 5 {
		t.Fatalf("expected 5 batches, got %d", snap.TotalBatches)
	}
	if snap.SuccessfulBatches != 5 || snap.FailedBatches != 0 {
		t.Fatalf("unexpected success/fail counts: %+v", snap)
	}
	if snap.TotalTargetsProcessed != 500 {
		t.Fatalf("expected 500 targets, got %d", snap.TotalTargetsProcessed)
	}
	if snap.StateChurnRatePct != 35.0 { // (35 mutated / 100) * 100%
		t.Fatalf("expected 35.0%% churn, got %.2f%%", snap.StateChurnRatePct)
	}
	if snap.P50DurationMs != 300.0 {
		t.Fatalf("expected P50 300ms, got %.1fms", snap.P50DurationMs)
	}
	if snap.SLABreached {
		t.Fatalf("expected SLABreached false")
	}

	prom := engine.FormatPrometheus()
	if !strings.Contains(prom, "recon_batch_total{status=\"success\"} 5") {
		t.Fatalf("missing success counter in prometheus: %s", prom)
	}
	if !strings.Contains(prom, "recon_state_churn_rate_pct 35.00") {
		t.Fatalf("missing churn rate in prometheus: %s", prom)
	}
}

func TestBatchTelemetryEngine_SLABreach(t *testing.T) {
	cfg := BatchTelemetryConfig{
		WindowSize:      10,
		MaxP99Duration:  500 * time.Millisecond,
		MaxErrorRatePct: 10.0,
	}
	engine := NewBatchTelemetryEngine(cfg)

	// 10 batches: 8 success (100ms), 2 failed (700ms)
	for i := 0; i < 8; i++ {
		engine.RecordBatch(BatchReconcileEvent{
			TargetsProcessed: 50,
			Duration:         100 * time.Millisecond,
			Success:          true,
		})
	}
	for i := 0; i < 2; i++ {
		engine.RecordBatch(BatchReconcileEvent{
			TargetsProcessed: 50,
			Duration:         700 * time.Millisecond,
			Success:          false,
			ErrorMessage:     "etcd leader election timeout",
		})
	}

	snap := engine.Snapshot()
	if snap.ErrorRatePct != 20.0 { // 2/10 = 20% > 10%
		t.Fatalf("expected 20%% error rate, got %.1f%%", snap.ErrorRatePct)
	}
	if !snap.SLABreached {
		t.Fatalf("expected SLABreached true")
	}
	if !strings.Contains(snap.BreachReason, "Error rate 20.00%") {
		t.Fatalf("unexpected breach reason: %s", snap.BreachReason)
	}
}
