package dispatch

import "testing"

func TestBenchmarkSnapshotDiff(t *testing.T) {
	d, elapsed := BenchmarkSnapshotDiff(map[string]string{"a": "1", "b": "2"}, map[string]string{"a": "1", "b": "3"}, 100)
	if d != 1 || elapsed < 0 {
		t.Fatalf("unexpected diff result: %d, %v", d, elapsed)
	}
}
