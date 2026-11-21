package core

import (
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestDependencyGraph_EmptyGraph(t *testing.T) {
	g := NewDependencyGraph()
	_, err := g.TopologicalSort()
	if !errors.Is(err, ErrEmptyGraph) {
		t.Fatalf("expected ErrEmptyGraph, got %v", err)
	}

	_, err = g.ComputeExecutionWaves()
	if !errors.Is(err, ErrEmptyGraph) {
		t.Fatalf("expected ErrEmptyGraph from ComputeExecutionWaves, got %v", err)
	}
}

func TestDependencyGraph_SelfDependency(t *testing.T) {
	g := NewDependencyGraph()
	err := g.AddDependency("service-a", "service-a")
	if !errors.Is(err, ErrSelfDependency) {
		t.Fatalf("expected ErrSelfDependency, got %v", err)
	}
}

func TestDependencyGraph_LinearPipeline(t *testing.T) {
	g := NewDependencyGraph()
	// web depends on api, api depends on db
	if err := g.AddDependency("web", "api"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddDependency("api", "db"); err != nil {
		t.Fatal(err)
	}

	waves, err := g.ComputeExecutionWaves()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedWaves := [][]string{
		{"db"},
		{"api"},
		{"web"},
	}

	if !reflect.DeepEqual(waves, expectedWaves) {
		t.Fatalf("expected waves %v, got %v", expectedWaves, waves)
	}

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedOrder := []string{"db", "api", "web"}
	if !reflect.DeepEqual(order, expectedOrder) {
		t.Fatalf("expected order %v, got %v", expectedOrder, order)
	}
}

func TestDependencyGraph_DiamondParallelWaves(t *testing.T) {
	g := NewDependencyGraph()
	// Sink depends on B and C; B and C both depend on Root
	_ = g.AddDependency("b", "root")
	_ = g.AddDependency("c", "root")
	_ = g.AddDependency("sink", "b")
	_ = g.AddDependency("sink", "c")

	waves, err := g.ComputeExecutionWaves()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedWaves := [][]string{
		{"root"},
		{"b", "c"},
		{"sink"},
	}

	if !reflect.DeepEqual(waves, expectedWaves) {
		t.Fatalf("expected waves %v, got %v", expectedWaves, waves)
	}
}

func TestDependencyGraph_CycleDetection(t *testing.T) {
	g := NewDependencyGraph()
	_ = g.AddDependency("node-1", "node-2")
	_ = g.AddDependency("node-2", "node-3")
	_ = g.AddDependency("node-3", "node-1")

	_, err := g.TopologicalSort()
	if !errors.Is(err, ErrCyclicDependency) {
		t.Fatalf("expected ErrCyclicDependency, got %v", err)
	}

	_, err = g.ComputeExecutionWaves()
	if !errors.Is(err, ErrCyclicDependency) {
		t.Fatalf("expected ErrCyclicDependency from waves, got %v", err)
	}
}

func TestDependencyGraph_Inspection(t *testing.T) {
	g := NewDependencyGraph()
	_ = g.AddDependency("app", "db")
	_ = g.AddDependency("app", "redis")

	deps, err := g.GetDependencies("app")
	if err != nil {
		t.Fatal(err)
	}
	expectedDeps := []string{"db", "redis"}
	if !reflect.DeepEqual(deps, expectedDeps) {
		t.Fatalf("expected dependencies %v, got %v", expectedDeps, deps)
	}

	dependents, err := g.GetDependents("db")
	if err != nil {
		t.Fatal(err)
	}
	expectedDependents := []string{"app"}
	if !reflect.DeepEqual(dependents, expectedDependents) {
		t.Fatalf("expected dependents %v, got %v", expectedDependents, dependents)
	}

	// Unknown node
	_, err = g.GetDependencies("nonexistent")
	if !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestDependencyGraph_Concurrency(t *testing.T) {
	g := NewDependencyGraph()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			_ = g.AddNode("base")
			_ = g.AddDependency("task", "base")
			_ = g.NodeCount()
			_, _ = g.GetDependencies("task")
		}(i)
	}

	wg.Wait()
	if g.NodeCount() != 2 {
		t.Fatalf("expected 2 unique nodes, got %d", g.NodeCount())
	}
}
