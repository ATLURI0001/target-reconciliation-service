package dispatch

import (
	"errors"
	"testing"
)

func TestDispatchCircuitBreaker(t *testing.T) {
	cb := NewDispatchCircuitBreaker(2)
	boom := errors.New("fail")
	_ = cb.Call(func() error { return boom })
	_ = cb.Call(func() error { return boom })
	if err := cb.Call(func() error { return nil }); err != ErrDispatchCircuitOpen {
		t.Fatalf("expected circuit open, got %v", err)
	}
}
