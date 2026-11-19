package core

import (
	"fmt"
	"sync"
	"testing"
)

func TestBloomFilterZeroFalseNegatives(t *testing.T) {
	n := uint64(500)
	bf, err := NewTargetBloomFilter(n, 0.01)
	if err != nil {
		t.Fatalf("unexpected error creating bloom filter: %v", err)
	}

	keys := make([]string, n)
	for i := uint64(0); i < n; i++ {
		keys[i] = fmt.Sprintf("cluster-alpha/ns-%d/deployment-%d", i%10, i)
		bf.Add(keys[i])
	}

	if bf.Count() != n {
		t.Errorf("expected count %d, got %d", n, bf.Count())
	}

	// Guarantee: Zero false negatives
	for _, k := range keys {
		if !bf.Contains(k) {
			t.Errorf("false negative detected for key: %s", k)
		}
	}
}

func TestBloomFilterConcurrentAccess(t *testing.T) {
	bf, err := NewTargetBloomFilter(1000, 0.05)
	if err != nil {
		t.Fatalf("failed to init bloom filter: %v", err)
	}

	var wg sync.WaitGroup
	workers := 8
	itemsPerWorker := 100

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < itemsPerWorker; i++ {
				k := fmt.Sprintf("worker-%d-item-%d", workerID, i)
				bf.Add(k)
				if !bf.Contains(k) {
					t.Errorf("expected worker key %s to be present", k)
				}
			}
		}(w)
	}

	wg.Wait()
	if bf.Count() != uint64(workers*itemsPerWorker) {
		t.Errorf("expected count %d, got %d", workers*itemsPerWorker, bf.Count())
	}
}

func TestBloomFilterInvalidParams(t *testing.T) {
	_, err := NewTargetBloomFilter(0, 0.01)
	if err == nil {
		t.Error("expected error for 0 expected elements")
	}

	_, err = NewTargetBloomFilter(100, 0.0)
	if err == nil {
		t.Error("expected error for 0.0 false positive rate")
	}

	_, err = NewTargetBloomFilter(100, 1.0)
	if err == nil {
		t.Error("expected error for 1.0 false positive rate")
	}
}

func TestReconcileBatchesExactMatch(t *testing.T) {
	records := []ReconcileRecord{
		{Key: "c1/n1/d1", Version: "v1", PayloadHash: "hash-aaa"},
		{Key: "c1/n1/d2", Version: "v1", PayloadHash: "hash-bbb"},
		{Key: "c1/n1/d3", Version: "v2", PayloadHash: "hash-ccc"},
	}

	summary, err := ReconcileBatches(records, records, 10)
	if err != nil {
		t.Fatalf("unexpected reconcile error: %v", err)
	}

	if summary.MatchedCount != 3 {
		t.Errorf("expected 3 matched, got %d", summary.MatchedCount)
	}
	if summary.DiscrepancyCount != 0 {
		t.Errorf("expected 0 discrepancies, got %d", summary.DiscrepancyCount)
	}
}

func TestReconcileBatchesDiscrepancyPartitioning(t *testing.T) {
	source := []ReconcileRecord{
		{Key: "c1/n1/app-common", Version: "v1", PayloadHash: "hash-1"},
		{Key: "c1/n1/app-modified", Version: "v1", PayloadHash: "hash-src-2"},
		{Key: "c1/n1/app-src-only", Version: "v1", PayloadHash: "hash-3"},
	}

	target := []ReconcileRecord{
		{Key: "c1/n1/app-common", Version: "v1", PayloadHash: "hash-1"},
		{Key: "c1/n1/app-modified", Version: "v2", PayloadHash: "hash-tgt-2"},
		{Key: "c1/n1/app-tgt-only", Version: "v1", PayloadHash: "hash-4"},
	}

	summary, err := ReconcileBatches(source, target, 10)
	if err != nil {
		t.Fatalf("unexpected reconcile error: %v", err)
	}

	if summary.MatchedCount != 1 {
		t.Errorf("expected 1 matched, got %d", summary.MatchedCount)
	}
	if summary.DiscrepancyCount != 3 {
		t.Errorf("expected 3 discrepancies, got %d", summary.DiscrepancyCount)
	}

	kinds := make(map[DiscrepancyKind]int)
	for _, d := range summary.Discrepancies {
		kinds[d.Kind]++
	}

	if kinds[DiscrepancyModified] != 1 {
		t.Errorf("expected 1 MODIFIED, got %d", kinds[DiscrepancyModified])
	}
	if kinds[DiscrepancyMissingInTarget] != 1 {
		t.Errorf("expected 1 MISSING_IN_TARGET, got %d", kinds[DiscrepancyMissingInTarget])
	}
	if kinds[DiscrepancyMissingInSource] != 1 {
		t.Errorf("expected 1 MISSING_IN_SOURCE, got %d", kinds[DiscrepancyMissingInSource])
	}
}

func TestReconcileBatchesMaxDiscrepanciesCapping(t *testing.T) {
	source := []ReconcileRecord{
		{Key: "k1", Version: "v1", PayloadHash: "h1"},
		{Key: "k2", Version: "v1", PayloadHash: "h2"},
		{Key: "k3", Version: "v1", PayloadHash: "h3"},
	}
	target := []ReconcileRecord{}

	summary, err := ReconcileBatches(source, target, 2)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}

	if summary.DiscrepancyCount != 3 {
		t.Errorf("expected 3 total discrepancies, got %d", summary.DiscrepancyCount)
	}
	if len(summary.Discrepancies) != 2 {
		t.Errorf("expected capped list of 2 items, got %d", len(summary.Discrepancies))
	}
}
