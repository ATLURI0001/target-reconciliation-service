package core

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ConvergenceHealthState categorizes the reconciliation velocity and stability.
type ConvergenceHealthState string

const (
	ConvergenceStateConverged ConvergenceHealthState = "CONVERGED"
	ConvergenceStateConverging ConvergenceHealthState = "CONVERGING"
	ConvergenceStateStalled   ConvergenceHealthState = "STALLED"
	ConvergenceStateDiverging ConvergenceHealthState = "DIVERGING"
)

var (
	ErrEmptyURN = errors.New("convergence: resource URN cannot be empty")
)

// ConvergenceCycleSample records the outcome of a single reconciliation pass.
type ConvergenceCycleSample struct {
	URN                  string    `json:"urn"`
	DurationMs           float64   `json:"duration_ms"`
	IterationsToConverge int       `json:"iterations_to_converge"`
	Success              bool      `json:"success"`
	DriftDeltaCount      int       `json:"drift_delta_count"`
	Timestamp            time.Time `json:"timestamp"`
}

// ResourceConvergenceDiagnostic provides per-resource convergence metrics.
type ResourceConvergenceDiagnostic struct {
	URN                  string                 `json:"urn"`
	SampleCount          int                    `json:"sample_count"`
	AvgDurationMs        float64                `json:"avg_duration_ms"`
	P95DurationMs        float64                `json:"p95_duration_ms"`
	AvgIterations        float64                `json:"avg_iterations"`
	SuccessRate          float64                `json:"success_rate"`
	LatestDriftDelta     int                    `json:"latest_drift_delta"`
	HealthState          ConvergenceHealthState `json:"health_state"`
	Advisories           []string               `json:"advisories,omitempty"`
}

// GlobalConvergenceDiagnostic provides aggregate cluster-wide reconciliation telemetry.
type GlobalConvergenceDiagnostic struct {
	TotalTrackedResources int                    `json:"total_tracked_resources"`
	TotalCyclesRecorded   int                    `json:"total_cycles_recorded"`
	OverallSuccessRate    float64                `json:"overall_success_rate"`
	GlobalP95DurationMs   float64                `json:"global_p95_duration_ms"`
	HealthyCount          int                    `json:"healthy_count"`
	StalledCount          int                    `json:"stalled_count"`
	DivergingCount        int                    `json:"diverging_count"`
}

// ConvergenceTelemetryEngine aggregates and analyzes reconciliation iteration cycles.
type ConvergenceTelemetryEngine struct {
	mu           sync.RWMutex
	historyLimit int
	history      map[string][]ConvergenceCycleSample
}

// NewConvergenceTelemetryEngine initializes a thread-safe convergence telemetry collector.
func NewConvergenceTelemetryEngine(historyLimit int) *ConvergenceTelemetryEngine {
	if historyLimit < 10 {
		historyLimit = 100
	}
	return &ConvergenceTelemetryEngine{
		historyLimit: historyLimit,
		history:      make(map[string][]ConvergenceCycleSample),
	}
}

// RecordCycle records a reconciliation cycle sample.
func (e *ConvergenceTelemetryEngine) RecordCycle(sample ConvergenceCycleSample) error {
	cleanURN := strings.TrimSpace(sample.URN)
	if cleanURN == "" {
		return ErrEmptyURN
	}

	if sample.Timestamp.IsZero() {
		sample.Timestamp = time.Now().UTC()
	}
	sample.URN = cleanURN

	e.mu.Lock()
	defer e.mu.Unlock()

	records := e.history[cleanURN]
	records = append(records, sample)
	if len(records) > e.historyLimit {
		records = records[len(records)-e.historyLimit:]
	}
	e.history[cleanURN] = records

	return nil
}

// GetResourceDiagnostic computes convergence statistics for a specific resource.
func (e *ConvergenceTelemetryEngine) GetResourceDiagnostic(urn string) (ResourceConvergenceDiagnostic, error) {
	cleanURN := strings.TrimSpace(urn)
	if cleanURN == "" {
		return ResourceConvergenceDiagnostic{}, ErrEmptyURN
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	records, exists := e.history[cleanURN]
	if !exists || len(records) == 0 {
		return ResourceConvergenceDiagnostic{
			URN:         cleanURN,
			HealthState: ConvergenceStateConverged,
		}, nil
	}

	return computeResourceStats(cleanURN, records), nil
}

// GetGlobalDiagnostic compiles cluster-wide reconciliation convergence metrics.
func (e *ConvergenceTelemetryEngine) GetGlobalDiagnostic() GlobalConvergenceDiagnostic {
	e.mu.RLock()
	defer e.mu.RUnlock()

	totalCycles := 0
	totalSuccess := 0
	var allDurations []float64

	healthy := 0
	stalled := 0
	diverging := 0

	for urn, records := range e.history {
		diag := computeResourceStats(urn, records)
		totalCycles += len(records)

		for _, r := range records {
			if r.Success {
				totalSuccess++
			}
			allDurations = append(allDurations, r.DurationMs)
		}

		switch diag.HealthState {
		case ConvergenceStateConverged, ConvergenceStateConverging:
			healthy++
		case ConvergenceStateStalled:
			stalled++
		case ConvergenceStateDiverging:
			diverging++
		}
	}

	successRate := 1.0
	if totalCycles > 0 {
		successRate = float64(totalSuccess) / float64(totalCycles)
	}

	p95Global := calculateP95(allDurations)

	return GlobalConvergenceDiagnostic{
		TotalTrackedResources: len(e.history),
		TotalCyclesRecorded:   totalCycles,
		OverallSuccessRate:    roundMetric(successRate),
		GlobalP95DurationMs:   roundMetric(p95Global),
		HealthyCount:          healthy,
		StalledCount:          stalled,
		DivergingCount:        diverging,
	}
}

// ExportPrometheusMetrics outputs Prometheus exposition text.
func (e *ConvergenceTelemetryEngine) ExportPrometheusMetrics() string {
	global := e.GetGlobalDiagnostic()
	var sb strings.Builder

	sb.WriteString("# HELP reconciler_convergence_cycles_total Total reconciliation cycles recorded\n")
	sb.WriteString("# TYPE reconciler_convergence_cycles_total counter\n")
	sb.WriteString(fmt.Sprintf("reconciler_convergence_cycles_total %d\n", global.TotalCyclesRecorded))

	sb.WriteString("# HELP reconciler_convergence_success_ratio Ratio of successful convergence passes\n")
	sb.WriteString("# TYPE reconciler_convergence_success_ratio gauge\n")
	sb.WriteString(fmt.Sprintf("reconciler_convergence_success_ratio %.4f\n", global.OverallSuccessRate))

	sb.WriteString("# HELP reconciler_convergence_p95_duration_ms Global P95 convergence duration\n")
	sb.WriteString("# TYPE reconciler_convergence_p95_duration_ms gauge\n")
	sb.WriteString(fmt.Sprintf("reconciler_convergence_p95_duration_ms %.2f\n", global.GlobalP95DurationMs))

	sb.WriteString("# HELP reconciler_resources_by_state Count of resources in each convergence state\n")
	sb.WriteString("# TYPE reconciler_resources_by_state gauge\n")
	sb.WriteString(fmt.Sprintf("reconciler_resources_by_state{state=\"healthy\"} %d\n", global.HealthyCount))
	sb.WriteString(fmt.Sprintf("reconciler_resources_by_state{state=\"stalled\"} %d\n", global.StalledCount))
	sb.WriteString(fmt.Sprintf("reconciler_resources_by_state{state=\"diverging\"} %d\n", global.DivergingCount))

	return sb.String()
}

func computeResourceStats(urn string, records []ConvergenceCycleSample) ResourceConvergenceDiagnostic {
	n := len(records)
	var durations []float64
	totalIter := 0
	successCount := 0

	for _, r := range records {
		durations = append(durations, r.DurationMs)
		totalIter += r.IterationsToConverge
		if r.Success {
			successCount++
		}
	}

	avgDur := 0.0
	for _, d := range durations {
		avgDur += d
	}
	avgDur /= float64(n)

	p95 := calculateP95(durations)
	avgIter := float64(totalIter) / float64(n)
	successRate := float64(successCount) / float64(n)

	latestSample := records[n-1]
	healthState := ConvergenceStateConverged
	var advisories []string

	if latestSample.DriftDeltaCount > 0 {
		healthState = ConvergenceStateConverging
	}

	// Divergence detection: drift delta increased across last 2 samples
	if n >= 2 {
		prevSample := records[n-2]
		if latestSample.DriftDeltaCount > prevSample.DriftDeltaCount && latestSample.DriftDeltaCount > 0 {
			healthState = ConvergenceStateDiverging
			advisories = append(advisories, "Drift delta count increased compared to previous cycle: reconciliation is diverging")
		}
	}

	// Stalled detection: success rate < 50% or consecutive iterations > 5
	if successRate < 0.5 || avgIter > 5.0 {
		if healthState != ConvergenceStateDiverging {
			healthState = ConvergenceStateStalled
		}
		advisories = append(advisories, "Low convergence velocity: multiple retries required or failing reconciliation loop")
	}

	return ResourceConvergenceDiagnostic{
		URN:              urn,
		SampleCount:      n,
		AvgDurationMs:    roundMetric(avgDur),
		P95DurationMs:    roundMetric(p95),
		AvgIterations:    roundMetric(avgIter),
		SuccessRate:      roundMetric(successRate),
		LatestDriftDelta: latestSample.DriftDeltaCount,
		HealthState:      healthState,
		Advisories:       advisories,
	}
}

func calculateP95(vals []float64) float64 {
	if len(vals) == 0 {
		return 0.0
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)

	idx := int(float64(len(sorted)) * 0.95)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func roundMetric(v float64) float64 {
	return float64(int(v*10000+0.5)) / 10000.0
}
