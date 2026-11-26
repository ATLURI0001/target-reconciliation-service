package dispatch

import (
	"crypto/sha256"
	"time"
)

// BenchmarkDiffHash measures SHA-256 diff payload hashing throughput.
func BenchmarkDiffHash(payload []byte, iterations int) time.Duration {
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_ = sha256.Sum256(payload)
	}
	return time.Since(start)
}
