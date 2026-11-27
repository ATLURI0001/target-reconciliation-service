package dispatch

import "testing"

func TestQueueDepthProfiler(t *testing.T) {
	p := NewQueueDepthProfiler()
	p.Record(3)
	p.Record(8)
	if p.Max() != 8 {
		t.Fatalf("expected 8, got %d", p.Max())
	}
}
