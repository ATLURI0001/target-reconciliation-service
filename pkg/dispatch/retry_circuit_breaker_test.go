package dispatch

import "testing"

func TestRetryCircuitBreaker(t *testing.T) {
	b := NewRetryCircuitBreaker(2)
	if b.IsOpen() {
		t.Fatal("expected closed initially")
	}
	b.RecordFailure()
	b.RecordFailure()
	if !b.IsOpen() {
		t.Fatal("expected open after 2 failures")
	}
}
