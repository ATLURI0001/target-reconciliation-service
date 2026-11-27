package dispatch

import (
	"time"
)

// RequeueProfiler measures requeue latency samples.
type RequeueProfiler struct {
	samples []time.Duration
}

func (p *RequeueProfiler) Record(start time.Time) {
	p.samples = append(p.samples, time.Since(start))
}

func (p *RequeueProfiler) Len() int { return len(p.samples) }
