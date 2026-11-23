package core

import (
	"errors"
	"math"
	"sort"
	"sync"
	"time"
)

// DriftHealthStatus categorizes the health of resource reconciliation drift and flapping.
type DriftHealthStatus string

const (
	DriftHealthy                 DriftHealthStatus = "HEALTHY"
	DriftWarningElevated         DriftHealthStatus = "WARNING_ELEVATED_DRIFT"
	DriftWarningFlapping         DriftHealthStatus = "WARNING_RESOURCE_FLAPPING"
	DriftCriticalControllerFight DriftHealthStatus = "CRITICAL_CONTROLLER_FIGHT"
)

var (
	// ErrEmptyResourceID is returned when an invalid resource ID is supplied to the profiler.
	ErrEmptyResourceID = errors.New("core: resource ID cannot be empty")
	// ErrNegativeLatency is returned when resolution duration is negative or non-finite.
	ErrNegativeLatency = errors.New("core: resolution latency cannot be negative")
)

// DriftEvent records an observed reconciliation event for a single managed resource.
type DriftEvent struct {
	ResourceID          string
	HasDrift            bool
	ResolutionLatencyMs float64
	DriftFields         []string
	Timestamp           time.Time
}

// DriftTelemetryConfig specifies threshold boundaries and retention settings.
type DriftTelemetryConfig struct {
	FlapWindow                 time.Duration
	FlapThreshold              int
	ElevatedDriftRateThreshold float64
	CriticalDriftRateThreshold float64
	CriticalFlappingCount      int
	MaxHistory                 int
	NowFunc                    func() time.Time
}

// DefaultDriftTelemetryConfig returns production-ready observability defaults.
func DefaultDriftTelemetryConfig() DriftTelemetryConfig {
	return DriftTelemetryConfig{
		FlapWindow:                 5 * time.Minute,
		FlapThreshold:              3,
		ElevatedDriftRateThreshold: 0.25,
		CriticalDriftRateThreshold: 0.50,
		CriticalFlappingCount:      5,
		MaxHistory:                 5000,
		NowFunc:                    time.Now,
	}
}

// DriftDiagnosticReport encapsulates statistical aggregates and automated operational insights.
type DriftDiagnosticReport struct {
	TotalEvaluated          int
	DriftedCount            int
	DriftRate               float64
	FlappingResources       []string
	AvgResolutionLatencyMs  float64
	P95ResolutionLatencyMs  float64
	Status                  DriftHealthStatus
	Recommendations         []string
}

// DriftTelemetryProfiler captures, indexes, and analyzes drift rates and resource flapping in real time.
type DriftTelemetryProfiler struct {
	mu          sync.RWMutex
	config      DriftTelemetryConfig
	history     []DriftEvent
	flapHistory map[string][]time.Time
}

// NewDriftTelemetryProfiler initializes a profiler with the given configuration.
func NewDriftTelemetryProfiler(cfg DriftTelemetryConfig) *DriftTelemetryProfiler {
	if cfg.FlapWindow <= 0 {
		cfg.FlapWindow = 5 * time.Minute
	}
	if cfg.FlapThreshold <= 0 {
		cfg.FlapThreshold = 3
	}
	if cfg.ElevatedDriftRateThreshold <= 0.0 {
		cfg.ElevatedDriftRateThreshold = 0.25
	}
	if cfg.CriticalDriftRateThreshold <= 0.0 {
		cfg.CriticalDriftRateThreshold = 0.50
	}
	if cfg.CriticalFlappingCount <= 0 {
		cfg.CriticalFlappingCount = 5
	}
	if cfg.MaxHistory <= 0 {
		cfg.MaxHistory = 5000
	}
	if cfg.NowFunc == nil {
		cfg.NowFunc = time.Now
	}

	return &DriftTelemetryProfiler{
		config:      cfg,
		history:     make([]DriftEvent, 0),
		flapHistory: make(map[string][]time.Time),
	}
}

// Record ingests a reconciliation outcome and registers state transitions for flap analysis.
func (p *DriftTelemetryProfiler) Record(evt DriftEvent) error {
	if evt.ResourceID == "" {
		return ErrEmptyResourceID
	}
	if evt.ResolutionLatencyMs < 0.0 || math.IsNaN(evt.ResolutionLatencyMs) {
		return ErrNegativeLatency
	}
	if evt.Timestamp.IsZero() {
		evt.Timestamp = p.config.NowFunc()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.history = append(p.history, evt)
	if len(p.history) > p.config.MaxHistory {
		p.history = p.history[1:]
	}

	if evt.HasDrift {
		p.flapHistory[evt.ResourceID] = append(p.flapHistory[evt.ResourceID], evt.Timestamp)
	}

	return nil
}

// Diagnose calculates aggregate drift velocity, flapping resources, and automated status recommendations.
func (p *DriftTelemetryProfiler) Diagnose() DriftDiagnosticReport {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := p.config.NowFunc()
	cutoff := now.Add(-p.config.FlapWindow)

	total := len(p.history)
	if total == 0 {
		return DriftDiagnosticReport{
			TotalEvaluated:         0,
			DriftedCount:           0,
			DriftRate:              0.0,
			FlappingResources:      []string{},
			AvgResolutionLatencyMs: 0.0,
			P95ResolutionLatencyMs: 0.0,
			Status:                 DriftHealthy,
			Recommendations:        []string{},
		}
	}

	driftedCount := 0
	var sumLatency float64
	latencies := make([]float64, 0, total)

	for _, evt := range p.history {
		if evt.HasDrift {
			driftedCount++
			sumLatency += evt.ResolutionLatencyMs
			latencies = append(latencies, evt.ResolutionLatencyMs)
		}
	}

	driftRate := float64(driftedCount) / float64(total)

	// Clean up flap history older than cutoff window and identify flapping resources
	var flapping []string
	for resID, times := range p.flapHistory {
		validIdx := 0
		for _, t := range times {
			if t.After(cutoff) {
				times[validIdx] = t
				validIdx++
			}
		}
		trimmed := times[:validIdx]
		if len(trimmed) == 0 {
			delete(p.flapHistory, resID)
		} else {
			p.flapHistory[resID] = trimmed
			if len(trimmed) >= p.config.FlapThreshold {
				flapping = append(flapping, resID)
			}
		}
	}
	sort.Strings(flapping)

	var avgLatency, p95Latency float64
	if len(latencies) > 0 {
		avgLatency = sumLatency / float64(len(latencies))
		sort.Float64s(latencies)
		p95Idx := int(float64(len(latencies)-1) * 0.95)
		p95Latency = latencies[p95Idx]
	}

	// Status determination and recommendations
	status := DriftHealthy
	var recommendations []string

	if len(flapping) >= p.config.CriticalFlappingCount || driftRate >= p.config.CriticalDriftRateThreshold {
		status = DriftCriticalControllerFight
		recommendations = append(recommendations,
			"Critical controller fight or systemic drift detected; immediately quarantine conflicting mutators and verify RBAC ownership.",
		)
	} else if len(flapping) > 0 {
		status = DriftWarningFlapping
		recommendations = append(recommendations,
			"Resource state flapping observed within observation window; inspect external webhooks or conflicting reconcile loops.",
		)
	} else if driftRate >= p.config.ElevatedDriftRateThreshold {
		status = DriftWarningElevated
		recommendations = append(recommendations,
			"Elevated reconciliation drift rate exceeds warning ceiling; audit out-of-band updates.",
		)
	}

	return DriftDiagnosticReport{
		TotalEvaluated:         total,
		DriftedCount:           driftedCount,
		DriftRate:              math.Round(driftRate*10000) / 10000,
		FlappingResources:      flapping,
		AvgResolutionLatencyMs: math.Round(avgLatency*100) / 100,
		P95ResolutionLatencyMs: math.Round(p95Latency*100) / 100,
		Status:                 status,
		Recommendations:        recommendations,
	}
}

// Reset clears the profiler history.
func (p *DriftTelemetryProfiler) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.history = make([]DriftEvent, 0)
	p.flapHistory = make(map[string][]time.Time)
}
