package dispatch

// ApplyDurationRotary tracks bounded state-apply duration samples in ms.
type ApplyDurationRotary struct {
	samples []float64
	cap     int
}

func NewApplyDurationRotary(capacity int) *ApplyDurationRotary {
	return &ApplyDurationRotary{cap: capacity}
}

func (r *ApplyDurationRotary) Record(ms float64) {
	if len(r.samples) >= r.cap {
		r.samples = r.samples[1:]
	}
	r.samples = append(r.samples, ms)
}

func (r *ApplyDurationRotary) Avg() float64 {
	if len(r.samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range r.samples {
		sum += s
	}
	return sum / float64(len(r.samples))
}
