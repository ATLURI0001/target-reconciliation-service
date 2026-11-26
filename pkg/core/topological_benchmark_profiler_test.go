package core

import (
	"errors"
	"testing"
)

func TestTopologicalBenchmarkProfiler_ResolveOrder(t *testing.T) {
	profiler := NewTopologicalBenchmarkProfiler()

	// Diamond DAG: A -> B, A -> C, B -> D, C -> D
	nodes := []GraphNode{
		{ID: "A", Dependencies: nil},
		{ID: "B", Dependencies: []string{"A"}},
		{ID: "C", Dependencies: []string{"A"}},
		{ID: "D", Dependencies: []string{"B", "C"}},
	}

	order, err := profiler.ResolveOrder(nodes)
	if err != nil {
		t.Fatalf("unexpected error resolving DAG: %v", err)
	}

	if len(order) != 4 {
		t.Fatalf("expected 4 nodes in resolved order, got %d", len(order))
	}
	if order[0] != "A" || order[3] != "D" {
		t.Fatalf("expected A first and D last, got %v", order)
	}
}

func TestTopologicalBenchmarkProfiler_CycleDetection(t *testing.T) {
	profiler := NewTopologicalBenchmarkProfiler()

	// Cycle: A -> B -> C -> A
	nodes := []GraphNode{
		{ID: "A", Dependencies: []string{"C"}},
		{ID: "B", Dependencies: []string{"A"}},
		{ID: "C", Dependencies: []string{"B"}},
	}

	_, err := profiler.ResolveOrder(nodes)
	if !errors.Is(err, ErrGraphCycleDetected) {
		t.Fatalf("expected ErrGraphCycleDetected, got %v", err)
	}
}

func TestTopologicalBenchmarkProfiler_BenchmarkRun(t *testing.T) {
	profiler := NewTopologicalBenchmarkProfiler()

	// Chain of 10 nodes: N0 -> N1 -> ... -> N9
	nodes := make([]GraphNode, 10)
	for i := 0; i < 10; i++ {
		var deps []string
		if i > 0 {
			deps = []string{string(rune('A' + i - 1))}
		}
		nodes[i] = GraphNode{ID: string(rune('A' + i)), Dependencies: deps}
	}

	res, err := profiler.RunBenchmark(nodes, 20)
	if err != nil {
		t.Fatalf("benchmark failed: %v", err)
	}

	if res.NodeCount != 10 {
		t.Errorf("expected 10 nodes, got %d", res.NodeCount)
	}
	if res.TotalRuns != 20 {
		t.Errorf("expected 20 runs, got %d", res.TotalRuns)
	}
	if res.P50Microseconds < 0 {
		t.Errorf("expected non-negative P50, got %f", res.P50Microseconds)
	}
	if res.ThroughputNodesSec <= 0 {
		t.Errorf("expected positive throughput, got %d", res.ThroughputNodesSec)
	}
}
