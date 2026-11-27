package dispatch

import (
	"errors"
	"testing"
	"time"
)

func TestLockLeaseCircuitBreaker(t *testing.T) {
	cb := NewLockLeaseCircuitBreaker(2, 5*time.Second)
	fail := func() error { return errors.New("etcd timeout") }
	_ = cb.Call(fail)
	_ = cb.Call(fail)
	if err := cb.Call(func() error { return nil }); err != ErrLockLeaseOpen {
		t.Fatalf("expected ErrLockLeaseOpen, got %v", err)
	}
}
