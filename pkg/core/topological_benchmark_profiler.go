package core

import (
	"errors"
	"math"
	"sort"
	"time"
)

var (
	ErrGraphCycleDetected = errors.New("topological: dependency cycle detected in graph")
)

// GraphNode represents an individual resource or reconciliation step.
type GraphNode struct {
	ID           string
	Dependencies []string
}

// TopologicalBenchmarkResult aggregates performance metrics for DAG evaluation.
type TopologicalBenchmarkResult struct {
	NodeCount         int
	EdgeCount         int
	TotalRuns         int
	P50Microseconds   float64
	P90Microseconds   float64
	P99Microseconds   float64
	ThroughputNodesSec int64
}

// TopologicalBenchmarkProfiler benchmarks DAG resolution and cycle detection.
type TopologicalBenchmarkProfiler struct{}

// NewTopologicalBenchmarkProfiler creates a new profiler instance.
func NewTopologicalBenchmarkProfiler() *TopologicalBenchmarkProfiler {
	return &TopologicalBenchmarkProfiler{}
}

// ResolveOrder resolves execution order using Kahn's algorithm.
func (p *TopologicalBenchmarkProfiler) ResolveOrder(nodes []GraphNode) ([]string, error) {
	inDegree := make(map[string]int, len(nodes))
	adjList := make(map[string][]string, len(nodes))

	for _, n := range nodes {
		if _, exists := inDegree[n.ID]; !exists {
			inDegree[n.ID] = 0
		}
		for _, dep := range n.Dependencies {
			adjList[dep] = append(adjList[dep], n.ID)
			inDegree[n.ID]++
		}
	}

	queue := make([]string, 0, len(nodes))
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	order := make([]string, 0, len(nodes))
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		order = append(order, curr)

		for _, neighbor := range adjList[curr] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(order) != len(inDegree) {
		return nil, ErrGraphCycleDetected
	}

	return order, nil
}

// RunBenchmark measures DAG resolution latency and throughput over multiple iterations.
func (p *TopologicalBenchmarkProfiler) RunBenchmark(nodes []GraphNode, iterations int) (TopologicalBenchmarkResult, error) {
	if iterations <= 0 {
		iterations = 50
	}

	// Validate graph is acyclic first
	if _, err := p.ResolveOrder(nodes); err != nil {
		return TopologicalBenchmarkResult{}, err
	}

	edgeCount := 0
	for _, n := range nodes {
		edgeCount += len(n.Dependencies)
	}

	latenciesMicros := make([]float64, iterations)
	totalStart := time.Now()
	innerBatch := 50

	for i := 0; i < iterations; i++ {
		start := time.Now()
		for b := 0; b < innerBatch; b++ {
			_, _ = p.ResolveOrder(nodes)
		}
		latenciesMicros[i] = float64(time.Since(start).Nanoseconds()) / 1000.0 / float64(innerBatch)
	}

	totalDurationSec := time.Since(totalStart).Seconds()
	sort.Float64s(latenciesMicros)

	p50 := latenciesMicros[int(float64(iterations)*0.5)]
	p90 := latenciesMicros[int(float64(iterations)*0.9)]
	p99 := latenciesMicros[int(float64(iterations)*0.99)]

	totalEvaluatedNodes := int64(len(nodes)) * int64(iterations) * int64(innerBatch)
	throughput := int64(0)
	if totalDurationSec > 0 {
		throughput = int64(float64(totalEvaluatedNodes) / totalDurationSec)
	}

	return TopologicalBenchmarkResult{
		NodeCount:          len(nodes),
		EdgeCount:          edgeCount,
		TotalRuns:          iterations,
		P50Microseconds:    math.Round(p50*100) / 100,
		P90Microseconds:    math.Round(p90*100) / 100,
		P99Microseconds:    math.Round(p99*100) / 100,
		ThroughputNodesSec: throughput,
	}, nil
}
