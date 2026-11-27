package dispatch

import "time"

// DispatchLatencyProfiler tracks dispatch operation latencies.
type DispatchLatencyProfiler struct {
	samples []time.Duration
}

func NewDispatchLatencyProfiler() *DispatchLatencyProfiler {
	return &DispatchLatencyProfiler{}
}

func (p *DispatchLatencyProfiler) Record(d time.Duration) {
	p.samples = append(p.samples, d)
}

func (p *DispatchLatencyProfiler) Max() time.Duration {
	var max time.Duration
	for _, s := range p.samples {
		if s > max {
			max = s
		}
	}
	return max
}
