package core

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// ReconciliationWorkload specifies synthetic plan execution parameters.
type ReconciliationWorkload struct {
	Name            string
	ActionCount     int
	BatchSize       int
	DependencyRatio float64
}

// PlanBenchmarkReport holds percentile latency metrics and throughput numbers.
type PlanBenchmarkReport struct {
	WorkloadName            string
	ActionCount             int
	BatchSize               int
	Iterations              int
	P50                     time.Duration
	P90                     time.Duration
	P99                     time.Duration
	Min                     time.Duration
	Max                     time.Duration
	Avg                     time.Duration
	ThroughputActionsPerSec int
}

// PlanExecutionBenchmarkProfiler measures DAG plan scheduling and reconciliation execution latency.
type PlanExecutionBenchmarkProfiler struct{}

// NewPlanExecutionBenchmarkProfiler constructs a new benchmark profiler.
func NewPlanExecutionBenchmarkProfiler() *PlanExecutionBenchmarkProfiler {
	return &PlanExecutionBenchmarkProfiler{}
}

// ExecuteSimulation executes a simulated reconciliation run across a synthetic workload.
func (p *PlanExecutionBenchmarkProfiler) ExecuteSimulation(w ReconciliationWorkload) (time.Duration, int, error) {
	if w.ActionCount <= 0 {
		return 0, 0, errors.New("actionCount must be positive")
	}
	if w.BatchSize <= 0 {
		w.BatchSize = 50
	}

	tStart := time.Now()

	// Simulate topological sorting & dependency resolution
	nodes := make([]int, w.ActionCount)
	for i := 0; i < w.ActionCount; i++ {
		nodes[i] = i
	}

	// Batching simulation
	batches := int(math.Ceil(float64(w.ActionCount) / float64(w.BatchSize)))
	processed := 0

	for b := 0; b < batches; b++ {
		start := b * w.BatchSize
		end := start + w.BatchSize
		if end > w.ActionCount {
			end = w.ActionCount
		}

		// Simulate batch validation and state hash computation
		for j := start; j < end; j++ {
			_ = nodes[j] * 31
			processed++
		}
	}

	duration := time.Since(tStart)
	if duration == 0 {
		duration = time.Microsecond
	}

	return duration, processed, nil
}

// ProfileWorkload executes multi-iteration profiling over a specified workload.
func (p *PlanExecutionBenchmarkProfiler) ProfileWorkload(
	w ReconciliationWorkload,
	iterations int,
	warmup int,
) (*PlanBenchmarkReport, error) {
	if iterations < 1 {
		return nil, errors.New("iterations must be at least 1")
	}

	// Warmup
	for i := 0; i < warmup; i++ {
		if _, _, err := p.ExecuteSimulation(w); err != nil {
			return nil, err
		}
	}

	latencies := make([]time.Duration, iterations)
	var totalTime time.Duration
	totalActions := 0

	for i := 0; i < iterations; i++ {
		dur, count, err := p.ExecuteSimulation(w)
		if err != nil {
			return nil, err
		}
		latencies[i] = dur
		totalTime += dur
		totalActions += count
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	n := len(latencies)
	getP := func(pct float64) time.Duration {
		idx := int(math.Floor((pct / 100.0) * float64(n)))
		if idx >= n {
			idx = n - 1
		}
		return latencies[idx]
	}

	throughput := 0
	if totalTime.Seconds() > 0 {
		throughput = int(float64(totalActions) / totalTime.Seconds())
	}

	return &PlanBenchmarkReport{
		WorkloadName:            w.Name,
		ActionCount:             w.ActionCount,
		BatchSize:               w.BatchSize,
		Iterations:              iterations,
		P50:                     getP(50),
		P90:                     getP(90),
		P99:                     getP(99),
		Min:                     latencies[0],
		Max:                     latencies[n-1],
		Avg:                     totalTime / time.Duration(n),
		ThroughputActionsPerSec: throughput,
	}, nil
}

// GenerateMarkdownMatrix formats a comparative benchmark report table.
func (p *PlanExecutionBenchmarkProfiler) GenerateMarkdownMatrix(reports []*PlanBenchmarkReport) string {
	var sb strings.Builder
	sb.WriteString("| Workload | Actions | Batch Size | p50 | p90 | p99 | Max | Throughput |\n")
	sb.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")

	for _, r := range reports {
		sb.WriteString(fmt.Sprintf(
			"| %s | %d | %d | %v | %v | %v | %v | %d act/s |\n",
			r.WorkloadName,
			r.ActionCount,
			r.BatchSize,
			r.P50,
			r.P90,
			r.P99,
			r.Max,
			r.ThroughputActionsPerSec,
		))
	}

	return sb.String()
}
