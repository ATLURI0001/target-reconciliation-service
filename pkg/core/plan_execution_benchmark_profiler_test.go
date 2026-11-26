package core

import (
	"strings"
	"testing"
)

func TestPlanExecutionBenchmarkProfiler_NominalSimulation(t *testing.T) {
	profiler := NewPlanExecutionBenchmarkProfiler()

	w := ReconciliationWorkload{
		Name:            "Batch-100",
		ActionCount:     100,
		BatchSize:       20,
		DependencyRatio: 0.1,
	}

	dur, processed, err := profiler.ExecuteSimulation(w)
	if err != nil {
		t.Fatalf("unexpected error during simulation: %v", err)
	}
	if processed != 100 {
		t.Fatalf("expected 100 processed actions, got %d", processed)
	}
	if dur <= 0 {
		t.Fatal("expected positive duration")
	}
}

func TestPlanExecutionBenchmarkProfiler_ProfileWorkload(t *testing.T) {
	profiler := NewPlanExecutionBenchmarkProfiler()

	w := ReconciliationWorkload{
		Name:        "Stress-500",
		ActionCount: 500,
		BatchSize:   50,
	}

	report, err := profiler.ProfileWorkload(w, 15, 2)
	if err != nil {
		t.Fatalf("failed to profile workload: %v", err)
	}

	if report.Iterations != 15 {
		t.Fatalf("expected 15 iterations, got %d", report.Iterations)
	}
	if report.ActionCount != 500 {
		t.Fatalf("expected 500 actions, got %d", report.ActionCount)
	}
	if report.P50 <= 0 {
		t.Fatal("expected positive p50 latency")
	}
	if report.ThroughputActionsPerSec <= 0 {
		t.Fatal("expected positive throughput")
	}
}

func TestPlanExecutionBenchmarkProfiler_MarkdownMatrix(t *testing.T) {
	profiler := NewPlanExecutionBenchmarkProfiler()

	w := ReconciliationWorkload{Name: "Quick-10", ActionCount: 10, BatchSize: 5}
	rep, _ := profiler.ProfileWorkload(w, 5, 1)

	md := profiler.GenerateMarkdownMatrix([]*PlanBenchmarkReport{rep})
	if !strings.Contains(md, "| Workload | Actions |") {
		t.Fatal("missing markdown table header")
	}
	if !strings.Contains(md, "Quick-10") {
		t.Fatal("missing workload name in markdown")
	}
}

func TestPlanExecutionBenchmarkProfiler_DefensiveValidation(t *testing.T) {
	profiler := NewPlanExecutionBenchmarkProfiler()

	_, _, err := profiler.ExecuteSimulation(ReconciliationWorkload{ActionCount: 0})
	if err == nil {
		t.Fatal("expected error on actionCount <= 0")
	}

	_, err = profiler.ProfileWorkload(ReconciliationWorkload{ActionCount: 10}, 0, 0)
	if err == nil {
		t.Fatal("expected error on iterations < 1")
	}
}
