package dispatch

import (
	"errors"
	"testing"
	"time"
)

func TestPlanDispatchCircuitBreaker(t *testing.T) {
	cb := NewPlanDispatchCircuitBreaker(2, 100*time.Millisecond)
	fail := func() error { return errors.New("dispatch error") }
	cb.Call(fail)
	cb.Call(fail)
	// Circuit should now be open
	if err := cb.Call(func() error { return nil }); err != ErrCircuitOpen {
		t.Fatalf("expected circuit open, got: %v", err)
	}
	// After cooldown, circuit closes
	time.Sleep(110 * time.Millisecond)
	if err := cb.Call(func() error { return nil }); err != nil {
		t.Fatalf("expected circuit closed after cooldown, got: %v", err)
	}
}
