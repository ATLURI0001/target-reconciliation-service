package core

import (
	"sort"
	"sync"
	"time"
)

// DriftSample captures resource state drift detection latency and diverged item counts.
type DriftSample struct {
	ResourceGroup string
	DriftedCount  int
	ScanMillis    int64
	Timestamp     time.Time
}

// DriftTelemetrySummary aggregates state drift frequency and scan latencies.
type DriftTelemetrySummary struct {
	SampleCount   int
	TotalDrifted  int
	P50ScanMillis int64
	P90ScanMillis int64
}

// DriftTelemetryRotary is a bounded circular ring buffer for state drift metrics.
type DriftTelemetryRotary struct {
	mu       sync.RWMutex
	capacity int
	buffer   []DriftSample
	head     int
	count    int
}

// NewDriftTelemetryRotary creates a new bounded telemetry rotary.
func NewDriftTelemetryRotary(capacity int) *DriftTelemetryRotary {
	if capacity <= 0 {
		capacity = 256
	}
	return &DriftTelemetryRotary{
		capacity: capacity,
		buffer:   make([]DriftSample, capacity),
	}
}

// Record stores a drift scan observation.
func (r *DriftTelemetryRotary) Record(group string, drifted int, scanMs int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buffer[r.head] = DriftSample{
		ResourceGroup: group,
		DriftedCount:  drifted,
		ScanMillis:    scanMs,
		Timestamp:     time.Now(),
	}
	r.head = (r.head + 1) % r.capacity
	if r.count < r.capacity {
		r.count++
	}
}

// GetSummary computes drift totals and scan latency quantiles.
func (r *DriftTelemetryRotary) GetSummary() DriftTelemetrySummary {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.count == 0 {
		return DriftTelemetrySummary{}
	}

	scans := make([]int64, r.count)
	totalDrifted := 0
	for i := 0; i < r.count; i++ {
		s := r.buffer[i]
		totalDrifted += s.DriftedCount
		scans[i] = s.ScanMillis
	}
	sort.Slice(scans, func(i, j int) bool { return scans[i] < scans[j] })

	return DriftTelemetrySummary{
		SampleCount:   r.count,
		TotalDrifted:  totalDrifted,
		P50ScanMillis: scans[int(float64(r.count)*0.5)],
		P90ScanMillis: scans[int(float64(r.count)*0.9)],
	}
}
