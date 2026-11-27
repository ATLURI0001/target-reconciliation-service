package dispatch

import "time"

// BenchmarkSnapshotDiff measures key-by-key map diff throughput.
func BenchmarkSnapshotDiff(a, b map[string]string, iterations int) (diffs int, elapsed time.Duration) {
	start := time.Now()
	for i := 0; i < iterations; i++ {
		diffs = 0
		for k, v := range a {
			if b[k] != v {
				diffs++
			}
		}
	}
	return diffs, time.Since(start)
}
