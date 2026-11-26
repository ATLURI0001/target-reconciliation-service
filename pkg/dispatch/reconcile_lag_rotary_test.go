package dispatch

import "testing"

func TestReconcileLagRotary(t *testing.T) {
	r := NewReconcileLagRotary(2)
	r.Record(5)
	r.Record(15)
	r.Record(25)
	if got := r.Avg(); got != 20 {
		t.Fatalf("expected 20, got %v", got)
	}
}
