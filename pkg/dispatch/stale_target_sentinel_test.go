package dispatch

import "testing"

func TestStaleTargetSentinel(t *testing.T) {
	s := NewStaleTargetSentinel(30)
	if !s.Check(10) {
		t.Fatal("expected fresh target to pass")
	}
	if s.Check(60) {
		t.Fatal("expected stale target to fail")
	}
	if s.StaleHits() != 1 {
		t.Fatalf("expected 1 hit, got %d", s.StaleHits())
	}
}
