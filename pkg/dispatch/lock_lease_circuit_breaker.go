package dispatch

import (
	"errors"
	"time"
)

var ErrLockLeaseOpen = errors.New("lock lease circuit breaker open")

// LockLeaseCircuitBreaker guards distributed lock lease renewals.
type LockLeaseCircuitBreaker struct {
	threshold int
	cooldown  time.Duration
	failures  int
	openUntil time.Time
}

func NewLockLeaseCircuitBreaker(threshold int, cooldown time.Duration) *LockLeaseCircuitBreaker {
	return &LockLeaseCircuitBreaker{threshold: threshold, cooldown: cooldown}
}

func (cb *LockLeaseCircuitBreaker) Call(fn func() error) error {
	if time.Now().Before(cb.openUntil) {
		return ErrLockLeaseOpen
	}
	if err := fn(); err != nil {
		cb.failures++
		if cb.failures >= cb.threshold {
			cb.openUntil = time.Now().Add(cb.cooldown)
			cb.failures = 0
		}
		return err
	}
	cb.failures = 0
	return nil
}
