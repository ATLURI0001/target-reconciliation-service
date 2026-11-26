package dispatch

// ReconcileLagRotary tracks bounded reconciliation lag samples in ms.
type ReconcileLagRotary struct {
	samples []float64
	cap     int
}

func NewReconcileLagRotary(capacity int) *ReconcileLagRotary {
	return &ReconcileLagRotary{cap: capacity}
}

func (r *ReconcileLagRotary) Record(ms float64) {
	if len(r.samples) >= r.cap {
		r.samples = r.samples[1:]
	}
	r.samples = append(r.samples, ms)
}

func (r *ReconcileLagRotary) Avg() float64 {
	if len(r.samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range r.samples {
		sum += s
	}
	return sum / float64(len(r.samples))
}
