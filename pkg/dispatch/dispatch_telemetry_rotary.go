package dispatch

// DispatchTelemetryRotary stores bounded dispatch count samples.
type DispatchTelemetryRotary struct {
	samples []int
	cap     int
}

func NewDispatchTelemetryRotary(cap int) *DispatchTelemetryRotary {
	if cap <= 0 {
		cap = 1
	}
	return &DispatchTelemetryRotary{cap: cap}
}

func (r *DispatchTelemetryRotary) Record(count int) {
	if len(r.samples) >= r.cap {
		r.samples = r.samples[1:]
	}
	r.samples = append(r.samples, count)
}

func (r *DispatchTelemetryRotary) Len() int { return len(r.samples) }
