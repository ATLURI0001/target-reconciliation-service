package dispatch

// StaleTargetSentinel detects targets with stale heartbeats.
type StaleTargetSentinel struct {
	maxAgeSec int64
	staleHits int
}

func NewStaleTargetSentinel(maxAgeSec int64) *StaleTargetSentinel {
	return &StaleTargetSentinel{maxAgeSec: maxAgeSec}
}

func (s *StaleTargetSentinel) Check(ageSec int64) bool {
	if ageSec > s.maxAgeSec {
		s.staleHits++
		return false
	}
	return true
}

func (s *StaleTargetSentinel) StaleHits() int {
	return s.staleHits
}
