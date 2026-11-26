package dispatch

import (
	"errors"
	"sync"
	"time"
)

// PlanDispatchCircuitBreaker protects the plan dispatch endpoint.
type PlanDispatchCircuitBreaker struct {
	mu          sync.Mutex
	failures    int
	threshold   int
	openUntil   time.Time
	cooldown    time.Duration
}

var ErrCircuitOpen = errors.New("circuit breaker open: plan dispatch endpoint unavailable")

// NewPlanDispatchCircuitBreaker creates a circuit breaker with given failure threshold and cooldown.
func NewPlanDispatchCircuitBreaker(threshold int, cooldown time.Duration) *PlanDispatchCircuitBreaker {
	return &PlanDispatchCircuitBreaker{threshold: threshold, cooldown: cooldown}
}

// Call executes fn under circuit breaker protection.
func (cb *PlanDispatchCircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	if time.Now().Before(cb.openUntil) {
		cb.mu.Unlock()
		return ErrCircuitOpen
	}
	cb.mu.Unlock()
	err := fn()
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if err != nil {
		cb.failures++
		if cb.failures >= cb.threshold {
			cb.openUntil = time.Now().Add(cb.cooldown)
			cb.failures = 0
		}
	} else {
		cb.failures = 0
	}
	return err
}
