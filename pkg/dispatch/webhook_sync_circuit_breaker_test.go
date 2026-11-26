package dispatch

import (
	"errors"
	"testing"
	"time"
)

func TestWebhookSyncCircuitBreaker(t *testing.T) {
	cb := NewWebhookSyncCircuitBreaker(2, 5*time.Second)
	fail := func() error { return errors.New("webhook 500") }
	_ = cb.Call(fail)
	_ = cb.Call(fail)
	if err := cb.Call(func() error { return nil }); err != ErrWebhookCircuitOpen {
		t.Fatalf("expected ErrWebhookCircuitOpen, got %v", err)
	}
}
