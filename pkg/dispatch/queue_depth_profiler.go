package dispatch

// QueueDepthProfiler tracks dispatch queue depth samples.
type QueueDepthProfiler struct {
	samples []int
}

func NewQueueDepthProfiler() *QueueDepthProfiler {
	return &QueueDepthProfiler{}
}

func (p *QueueDepthProfiler) Record(depth int) {
	p.samples = append(p.samples, depth)
}

func (p *QueueDepthProfiler) Max() int {
	max := 0
	for _, s := range p.samples {
		if s > max {
			max = s
		}
	}
	return max
}
