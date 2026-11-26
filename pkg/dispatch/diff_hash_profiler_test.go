package dispatch

import "testing"

func TestBenchmarkDiffHash(t *testing.T) {
	d := BenchmarkDiffHash([]byte(`{"op":"replace"}`), 5000)
	if d < 0 {
		t.Fatalf("expected non-negative duration, got %v", d)
	}
}
