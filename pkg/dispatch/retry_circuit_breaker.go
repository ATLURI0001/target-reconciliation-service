package dispatch

// RetryCircuitBreaker trips after consecutive dispatch failures.
type RetryCircuitBreaker struct {
	failures  int
	threshold int
	open      bool
}

func NewRetryCircuitBreaker(threshold int) *RetryCircuitBreaker {
	return &RetryCircuitBreaker{threshold: threshold}
}

func (b *RetryCircuitBreaker) RecordFailure() {
	b.failures++
	if b.failures >= b.threshold {
		b.open = true
	}
}

func (b *RetryCircuitBreaker) IsOpen() bool {
	return b.open
}
