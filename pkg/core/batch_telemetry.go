package core

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type BatchReconcileEvent struct {
	BatchID          string        `json:"batch_id"`
	TargetsProcessed int           `json:"targets_processed"`
	CreatedCount     int           `json:"created_count"`
	UpdatedCount     int           `json:"updated_count"`
	DeletedCount     int           `json:"deleted_count"`
	UnchangedCount   int           `json:"unchanged_count"`
	Duration         time.Duration `json:"duration"`
	Success          bool          `json:"success"`
	ErrorMessage     string        `json:"error_message,omitempty"`
	Timestamp        time.Time     `json:"timestamp"`
}

type BatchTelemetryConfig struct {
	WindowSize       int           `json:"window_size"`
	MaxP99Duration   time.Duration `json:"max_p99_duration"`
	MaxErrorRatePct  float64       `json:"max_error_rate_pct"`
	MaxStateChurnPct float64       `json:"max_state_churn_pct"`
}

func DefaultBatchTelemetryConfig() BatchTelemetryConfig {
	return BatchTelemetryConfig{
		WindowSize:       500,
		MaxP99Duration:   5 * time.Second,
		MaxErrorRatePct:  5.0,
		MaxStateChurnPct: 80.0,
	}
}

type BatchTelemetrySnapshot struct {
	TotalBatches           int           `json:"total_batches"`
	SuccessfulBatches      int           `json:"successful_batches"`
	FailedBatches          int           `json:"failed_batches"`
	ErrorRatePct           float64       `json:"error_rate_pct"`
	TotalTargetsProcessed  int           `json:"total_targets_processed"`
	TotalCreated           int           `json:"total_created"`
	TotalUpdated           int           `json:"total_updated"`
	TotalDeleted           int           `json:"total_deleted"`
	TotalUnchanged         int           `json:"total_unchanged"`
	StateChurnRatePct      float64       `json:"state_churn_rate_pct"`
	MeanDurationMs         float64       `json:"mean_duration_ms"`
	P50DurationMs          float64       `json:"p50_duration_ms"`
	P90DurationMs          float64       `json:"p90_duration_ms"`
	P99DurationMs          float64       `json:"p99_duration_ms"`
	SLABreached            bool          `json:"sla_breached"`
	BreachReason           string        `json:"breach_reason,omitempty"`
}

type BatchTelemetryEngine struct {
	mu     sync.RWMutex
	config BatchTelemetryConfig
	events []BatchReconcileEvent
}

func NewBatchTelemetryEngine(cfg BatchTelemetryConfig) *BatchTelemetryEngine {
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = 500
	}
	if cfg.MaxP99Duration <= 0 {
		cfg.MaxP99Duration = 5 * time.Second
	}
	if cfg.MaxErrorRatePct <= 0 {
		cfg.MaxErrorRatePct = 5.0
	}
	if cfg.MaxStateChurnPct <= 0 {
		cfg.MaxStateChurnPct = 80.0
	}

	return &BatchTelemetryEngine{
		config: cfg,
		events: make([]BatchReconcileEvent, 0, cfg.WindowSize),
	}
}

func (e *BatchTelemetryEngine) RecordBatch(ev BatchReconcileEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if ev.BatchID == "" {
		ev.BatchID = fmt.Sprintf("batch-%d", time.Now().UnixNano())
	}
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}
	if ev.Duration < 0 {
		ev.Duration = 0
	}

	e.events = append(e.events, ev)
	if len(e.events) > e.config.WindowSize {
		e.events = e.events[1:]
	}
}

func (e *BatchTelemetryEngine) Snapshot() BatchTelemetrySnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	n := len(e.events)
	if n == 0 {
		return BatchTelemetrySnapshot{}
	}

	var successful, failed int
	var totalTargets, created, updated, deleted, unchanged int
	var sumDuration time.Duration
	durations := make([]float64, 0, n)

	for _, ev := range e.events {
		if ev.Success {
			successful++
		} else {
			failed++
		}

		totalTargets += ev.TargetsProcessed
		created += ev.CreatedCount
		updated += ev.UpdatedCount
		deleted += ev.DeletedCount
		unchanged += ev.UnchangedCount

		sumDuration += ev.Duration
		durations = append(durations, float64(ev.Duration.Milliseconds()))
	}

	errorRate := (float64(failed) / float64(n)) * 100.0
	mutated := created + updated + deleted
	var churnRate float64
	if totalTargets > 0 {
		churnRate = (float64(mutated) / float64(totalTargets)) * 100.0
	}

	sort.Float64s(durations)
	p50 := durations[int(float64(n)*0.50)]
	p90 := durations[int(float64(n)*0.90)]
	p99 := durations[int(float64(n)*0.99)]
	meanMs := float64(sumDuration.Milliseconds()) / float64(n)

	var slaBreached bool
	var breachReason string

	if n >= 5 && errorRate > e.config.MaxErrorRatePct {
		slaBreached = true
		breachReason = fmt.Sprintf("Error rate %.2f%% exceeds threshold %.2f%%", errorRate, e.config.MaxErrorRatePct)
	} else if p99 > float64(e.config.MaxP99Duration.Milliseconds()) {
		slaBreached = true
		breachReason = fmt.Sprintf("P99 batch duration %.1fms exceeds SLA limit %dms", p99, e.config.MaxP99Duration.Milliseconds())
	}

	return BatchTelemetrySnapshot{
		TotalBatches:          n,
		SuccessfulBatches:     successful,
		FailedBatches:         failed,
		ErrorRatePct:          errorRate,
		TotalTargetsProcessed: totalTargets,
		TotalCreated:          created,
		TotalUpdated:          updated,
		TotalDeleted:          deleted,
		TotalUnchanged:        unchanged,
		StateChurnRatePct:     churnRate,
		MeanDurationMs:        meanMs,
		P50DurationMs:         p50,
		P90DurationMs:         p90,
		P99DurationMs:         p99,
		SLABreached:           slaBreached,
		BreachReason:          breachReason,
	}
}

func (e *BatchTelemetryEngine) FormatPrometheus() string {
	snap := e.Snapshot()
	var sb strings.Builder

	sb.WriteString("# HELP recon_batch_total Total reconciliation batches executed\n")
	sb.WriteString("# TYPE recon_batch_total counter\n")
	sb.WriteString(fmt.Sprintf("recon_batch_total{status=\"success\"} %d\n", snap.SuccessfulBatches))
	sb.WriteString(fmt.Sprintf("recon_batch_total{status=\"failed\"} %d\n", snap.FailedBatches))

	sb.WriteString("# HELP recon_targets_processed_total Total resource targets processed\n")
	sb.WriteString("# TYPE recon_targets_processed_total counter\n")
	sb.WriteString(fmt.Sprintf("recon_targets_processed_total{action=\"create\"} %d\n", snap.TotalCreated))
	sb.WriteString(fmt.Sprintf("recon_targets_processed_total{action=\"update\"} %d\n", snap.TotalUpdated))
	sb.WriteString(fmt.Sprintf("recon_targets_processed_total{action=\"delete\"} %d\n", snap.TotalDeleted))
	sb.WriteString(fmt.Sprintf("recon_targets_processed_total{action=\"noop\"} %d\n", snap.TotalUnchanged))

	sb.WriteString("# HELP recon_state_churn_rate_pct Percentage of targets undergoing state mutation\n")
	sb.WriteString("# TYPE recon_state_churn_rate_pct gauge\n")
	sb.WriteString(fmt.Sprintf("recon_state_churn_rate_pct %.2f\n", snap.StateChurnRatePct))

	sb.WriteString("# HELP recon_batch_duration_p99_ms 99th percentile batch reconciliation duration in ms\n")
	sb.WriteString("# TYPE recon_batch_duration_p99_ms gauge\n")
	sb.WriteString(fmt.Sprintf("recon_batch_duration_p99_ms %.2f\n", snap.P99DurationMs))

	return sb.String()
}
