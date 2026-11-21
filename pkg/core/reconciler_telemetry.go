package core

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// HealthState represents the operational status of the reconciliation pipeline.
type HealthState string

const (
	HealthStateHealthy   HealthState = "HEALTHY"
	HealthStateDegraded  HealthState = "DEGRADED"
	HealthStateUnhealthy HealthState = "UNHEALTHY"
)

// TelemetryConfig configures thresholds for health evaluation and metrics aggregation.
type TelemetryConfig struct {
	WindowSize             int           // Size of rolling execution buffer (default: 50)
	DegradedErrorThreshold float64       // Error rate above which state is DEGRADED (e.g. 0.20)
	UnhealthyErrorThreshold float64      // Error rate above which state is UNHEALTHY (e.g. 0.50)
	DegradedConsecutiveFailures int      // Consecutive failures threshold for DEGRADED (default: 3)
	UnhealthyConsecutiveFailures int     // Consecutive failures threshold for UNHEALTHY (default: 5)
	StaleThreshold         time.Duration // Time since last execution before flagging stale (default: 10m)
}

// DefaultTelemetryConfig provides safe defaults for telemetry aggregation.
func DefaultTelemetryConfig() TelemetryConfig {
	return TelemetryConfig{
		WindowSize:                  50,
		DegradedErrorThreshold:      0.20,
		UnhealthyErrorThreshold:     0.50,
		DegradedConsecutiveFailures: 3,
		UnhealthyConsecutiveFailures: 5,
		StaleThreshold:              10 * time.Minute,
	}
}

// ReconciliationMetricsSnapshot captures a point-in-time state of reconciliation telemetry.
type ReconciliationMetricsSnapshot struct {
	TotalRuns           uint64        `json:"total_runs"`
	SuccessCount        uint64        `json:"success_count"`
	FailureCount        uint64        `json:"failure_count"`
	DriftDetectedCount  uint64        `json:"drift_detected_count"`
	RollbackCount       uint64        `json:"rollback_count"`
	ConsecutiveFailures int           `json:"consecutive_failures"`
	RollingErrorRate    float64       `json:"rolling_error_rate"`
	TotalDuration       time.Duration `json:"total_duration"`
	AverageDuration     time.Duration `json:"average_duration"`
	MaxDuration         time.Duration `json:"max_duration"`
	LastDuration        time.Duration `json:"last_duration"`
	LastRunTime         time.Time     `json:"last_run_time"`
	Health              HealthState   `json:"health"`
	HealthReason        string        `json:"health_reason"`
}

// ReconcilerTelemetryCollector provides thread-safe collection and diagnostic health probing.
type ReconcilerTelemetryCollector struct {
	mu                  sync.RWMutex
	cfg                 TelemetryConfig
	startTime           time.Time
	totalRuns           uint64
	successCount        uint64
	failureCount        uint64
	driftDetectedCount  uint64
	rollbackCount       uint64
	consecutiveFailures int
	totalDuration       time.Duration
	maxDuration         time.Duration
	lastDuration        time.Duration
	lastRunTime         time.Time
	historyWindow       []bool // true = success, false = failure
	windowIdx           int
	windowFilled        bool
}

// NewReconcilerTelemetryCollector constructs a new collector with provided configuration.
func NewReconcilerTelemetryCollector(cfg TelemetryConfig) *ReconcilerTelemetryCollector {
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = 50
	}
	if cfg.DegradedErrorThreshold <= 0 {
		cfg.DegradedErrorThreshold = 0.20
	}
	if cfg.UnhealthyErrorThreshold <= 0 {
		cfg.UnhealthyErrorThreshold = 0.50
	}
	if cfg.DegradedConsecutiveFailures <= 0 {
		cfg.DegradedConsecutiveFailures = 3
	}
	if cfg.UnhealthyConsecutiveFailures <= 0 {
		cfg.UnhealthyConsecutiveFailures = 5
	}
	if cfg.StaleThreshold <= 0 {
		cfg.StaleThreshold = 10 * time.Minute
	}

	return &ReconcilerTelemetryCollector{
		cfg:           cfg,
		startTime:     time.Now(),
		historyWindow: make([]bool, cfg.WindowSize),
	}
}

// RecordExecution registers the result and latency of a single reconciliation cycle.
func (c *ReconcilerTelemetryCollector) RecordExecution(success bool, duration time.Duration, driftDetected bool, rollback bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.totalRuns++
	c.lastDuration = duration
	c.totalDuration += duration
	c.lastRunTime = time.Now()

	if duration > c.maxDuration {
		c.maxDuration = duration
	}

	if success {
		c.successCount++
		c.consecutiveFailures = 0
	} else {
		c.failureCount++
		c.consecutiveFailures++
	}

	if driftDetected {
		c.driftDetectedCount++
	}
	if rollback {
		c.rollbackCount++
	}

	// Update circular history window
	c.historyWindow[c.windowIdx] = success
	c.windowIdx++
	if c.windowIdx >= c.cfg.WindowSize {
		c.windowIdx = 0
		c.windowFilled = true
	}
}

// Snapshot returns an immutable point-in-time telemetry snapshot and health classification.
func (c *ReconcilerTelemetryCollector) Snapshot() ReconciliationMetricsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	errRate := c.computeRollingErrorRateLocked()
	health, reason := c.evaluateHealthLocked(errRate)

	avgDuration := time.Duration(0)
	if c.totalRuns > 0 {
		avgDuration = time.Duration(int64(c.totalDuration) / int64(c.totalRuns))
	}

	return ReconciliationMetricsSnapshot{
		TotalRuns:           c.totalRuns,
		SuccessCount:        c.successCount,
		FailureCount:        c.failureCount,
		DriftDetectedCount:  c.driftDetectedCount,
		RollbackCount:       c.rollbackCount,
		ConsecutiveFailures: c.consecutiveFailures,
		RollingErrorRate:    errRate,
		TotalDuration:       c.totalDuration,
		AverageDuration:     avgDuration,
		MaxDuration:         c.maxDuration,
		LastDuration:        c.lastDuration,
		LastRunTime:         c.lastRunTime,
		Health:              health,
		HealthReason:        reason,
	}
}

func (c *ReconcilerTelemetryCollector) computeRollingErrorRateLocked() float64 {
	totalSamples := c.windowIdx
	if c.windowFilled {
		totalSamples = c.cfg.WindowSize
	}
	if totalSamples == 0 {
		return 0.0
	}

	failures := 0
	for i := 0; i < totalSamples; i++ {
		if !c.historyWindow[i] {
			failures++
		}
	}

	return float64(failures) / float64(totalSamples)
}

func (c *ReconcilerTelemetryCollector) evaluateHealthLocked(errRate float64) (HealthState, string) {
	if c.totalRuns == 0 {
		return HealthStateHealthy, "Pipeline initialized; awaiting first run"
	}

	if c.consecutiveFailures >= c.cfg.UnhealthyConsecutiveFailures {
		return HealthStateUnhealthy, fmt.Sprintf("Consecutive failures (%d) reached unhealthy threshold (%d)",
			c.consecutiveFailures, c.cfg.UnhealthyConsecutiveFailures)
	}

	if errRate >= c.cfg.UnhealthyErrorThreshold {
		return HealthStateUnhealthy, fmt.Sprintf("Rolling error rate (%.2f%%) exceeds unhealthy limit (%.2f%%)",
			errRate*100, c.cfg.UnhealthyErrorThreshold*100)
	}

	if c.consecutiveFailures >= c.cfg.DegradedConsecutiveFailures {
		return HealthStateDegraded, fmt.Sprintf("Consecutive failures (%d) reached degraded threshold (%d)",
			c.consecutiveFailures, c.cfg.DegradedConsecutiveFailures)
	}

	if errRate >= c.cfg.DegradedErrorThreshold {
		return HealthStateDegraded, fmt.Sprintf("Rolling error rate (%.2f%%) exceeds degraded limit (%.2f%%)",
			errRate*100, c.cfg.DegradedErrorThreshold*100)
	}

	if !c.lastRunTime.IsZero() && time.Since(c.lastRunTime) > c.cfg.StaleThreshold {
		return HealthStateDegraded, fmt.Sprintf("Pipeline stale: last execution was %v ago (threshold: %v)",
			time.Since(c.lastRunTime).Round(time.Second), c.cfg.StaleThreshold)
	}

	return HealthStateHealthy, "Pipeline operating within normal thresholds"
}

// ExportPrometheusMetrics formats active telemetry counters and gauges in Prometheus text exposition format.
func (c *ReconcilerTelemetryCollector) ExportPrometheusMetrics() string {
	snap := c.Snapshot()

	var sb strings.Builder
	sb.WriteString("# HELP reconciler_total_runs_total Total reconciliation loop executions\n")
	sb.WriteString("# TYPE reconciler_total_runs_total counter\n")
	sb.WriteString(fmt.Sprintf("reconciler_total_runs_total %d\n", snap.TotalRuns))

	sb.WriteString("# HELP reconciler_success_total Total successful reconciliations\n")
	sb.WriteString("# TYPE reconciler_success_total counter\n")
	sb.WriteString(fmt.Sprintf("reconciler_success_total %d\n", snap.SuccessCount))

	sb.WriteString("# HELP reconciler_failures_total Total failed reconciliations\n")
	sb.WriteString("# TYPE reconciler_failures_total counter\n")
	sb.WriteString(fmt.Sprintf("reconciler_failures_total %d\n", snap.FailureCount))

	sb.WriteString("# HELP reconciler_drift_detected_total Total drift anomalies detected\n")
	sb.WriteString("# TYPE reconciler_drift_detected_total counter\n")
	sb.WriteString(fmt.Sprintf("reconciler_drift_detected_total %d\n", snap.DriftDetectedCount))

	sb.WriteString("# HELP reconciler_rollbacks_total Total compensatory rollbacks executed\n")
	sb.WriteString("# TYPE reconciler_rollbacks_total counter\n")
	sb.WriteString(fmt.Sprintf("reconciler_rollbacks_total %d\n", snap.RollbackCount))

	sb.WriteString("# HELP reconciler_rolling_error_rate Rolling error rate in current window\n")
	sb.WriteString("# TYPE reconciler_rolling_error_rate gauge\n")
	sb.WriteString(fmt.Sprintf("reconciler_rolling_error_rate %.4f\n", snap.RollingErrorRate))

	sb.WriteString("# HELP reconciler_duration_seconds Latency of last reconciliation in seconds\n")
	sb.WriteString("# TYPE reconciler_duration_seconds gauge\n")
	sb.WriteString(fmt.Sprintf("reconciler_duration_seconds %.6f\n", snap.LastDuration.Seconds()))

	sb.WriteString("# HELP reconciler_health_status Status enum (1=Healthy, 2=Degraded, 3=Unhealthy)\n")
	sb.WriteString("# TYPE reconciler_health_status gauge\n")
	statusVal := 1
	switch snap.Health {
	case HealthStateDegraded:
		statusVal = 2
	case HealthStateUnhealthy:
		statusVal = 3
	}
	sb.WriteString(fmt.Sprintf("reconciler_health_status %d\n", statusVal))

	return sb.String()
}
