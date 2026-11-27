package dispatch

import (
	"testing"
	"time"
)

func TestDispatchLatencyProfiler(t *testing.T) {
	p := NewDispatchLatencyProfiler()
	p.Record(5 * time.Millisecond)
	p.Record(15 * time.Millisecond)
	if p.Max() != 15*time.Millisecond {
		t.Fatalf("expected 15ms max, got %v", p.Max())
	}
}
