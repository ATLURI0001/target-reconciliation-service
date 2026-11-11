package tests

import (
	"context"
	"fmt"
	"github.com/ATLURI0001/resilienceplane/internal/core"

	"github.com/ATLURI0001/resilienceplane/internal/store"

	"testing"
)

// BenchmarkMemoryInventory1000 measures only in-process snapshot copy cost.
func BenchmarkMemoryInventory1000(b *testing.B) {
	db := store.NewMemory()
	_ = db.Update(context.Background(), func(s *core.State) error {
		for i := 0; i < 1000; i++ {
			o := core.Observation{Cluster: "bench", Name: fmt.Sprintf("deployment-%d", i), Namespace: "rp-demo", Desired: 1}
			s.Observations[o.Key()] = o
		}
		return nil
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s, e := db.View(context.Background())
		if e != nil || len(s.Observations) != 1000 {
			b.Fatal(e)
		}
	}
}
