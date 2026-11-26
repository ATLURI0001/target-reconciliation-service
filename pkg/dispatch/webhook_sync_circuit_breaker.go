package dispatch

import (
	"errors"
	"time"
)

var ErrWebhookCircuitOpen = errors.New("webhook sync circuit breaker open")

// WebhookSyncCircuitBreaker protects outbound reconciliation webhook calls.
type WebhookSyncCircuitBreaker struct {
	threshold int
	cooldown  time.Duration
	failures  int
	openUntil time.Time
}

func NewWebhookSyncCircuitBreaker(threshold int, cooldown time.Duration) *WebhookSyncCircuitBreaker {
	return &WebhookSyncCircuitBreaker{threshold: threshold, cooldown: cooldown}
}

func (cb *WebhookSyncCircuitBreaker) Call(fn func() error) error {
	if time.Now().Before(cb.openUntil) {
		return ErrWebhookCircuitOpen
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
