package dispatch

import "testing"

func TestDispatchTelemetryRotary(t *testing.T) {
	r := NewDispatchTelemetryRotary(3)
	for i := 0; i < 5; i++ {
		r.Record(i)
	}
	if r.Len() != 3 {
		t.Fatalf("expected 3, got %d", r.Len())
	}
}
