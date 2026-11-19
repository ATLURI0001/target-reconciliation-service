package core

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"sync"
	"time"
)

// TargetBloomFilter provides a thread-safe, space-efficient membership filter
// used to fast-path discrepancy detection between source and target datasets.
type TargetBloomFilter struct {
	mu     sync.RWMutex
	bits   []uint64
	m      uint64 // bitset capacity
	k      uint32 // number of hash functions
	count  uint64 // elements added
}

// NewTargetBloomFilter creates an optimal Bloom filter for expected elements and target false positive rate.
func NewTargetBloomFilter(expectedElements uint64, falsePositiveRate float64) (*TargetBloomFilter, error) {
	if expectedElements == 0 {
		return nil, fmt.Errorf("%w: expected elements must be greater than zero", ErrInvalid)
	}
	if falsePositiveRate <= 0.0 || falsePositiveRate >= 1.0 {
		return nil, fmt.Errorf("%w: false positive rate must be strictly between 0.0 and 1.0", ErrInvalid)
	}

	// m = - (n * ln(p)) / (ln(2)^2)
	ln2 := math.Ln2
	mFloat := -float64(expectedElements) * math.Log(falsePositiveRate) / (ln2 * ln2)
	m := uint64(math.Ceil(mFloat))
	if m < 64 {
		m = 64
	}

	// k = (m / n) * ln(2)
	kFloat := (float64(m) / float64(expectedElements)) * ln2
	k := uint32(math.Max(1, math.Round(kFloat)))

	words := (m + 63) / 64
	return &TargetBloomFilter{
		bits: make([]uint64, words),
		m:    m,
		k:    k,
	}, nil
}

func (bf *TargetBloomFilter) hashValues(key string) (uint64, uint64) {
	h1 := fnv.New64a()
	_, _ = h1.Write([]byte(key))
	v1 := h1.Sum64()

	// Secondary hash using multiplicative mixer
	v2 := v1*0x9e3779b97f4a7c15 + 0xbf58476d1ce4e5b9
	if v2 == 0 {
		v2 = 1
	}
	return v1, v2
}

// Add inserts a key into the Bloom filter.
func (bf *TargetBloomFilter) Add(key string) {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	h1, h2 := bf.hashValues(key)
	for i := uint32(0); i < bf.k; i++ {
		bitIdx := (h1 + uint64(i)*h2) % bf.m
		word := bitIdx / 64
		mask := uint64(1) << (bitIdx % 64)
		bf.bits[word] |= mask
	}
	bf.count++
}

// Contains checks whether a key might be in the filter.
// Guarantees zero false negatives: if it returns false, the key is 100% absent.
func (bf *TargetBloomFilter) Contains(key string) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	h1, h2 := bf.hashValues(key)
	for i := uint32(0); i < bf.k; i++ {
		bitIdx := (h1 + uint64(i)*h2) % bf.m
		word := bitIdx / 64
		mask := uint64(1) << (bitIdx % 64)
		if (bf.bits[word] & mask) == 0 {
			return false
		}
	}
	return true
}

// Capacity returns the total number of bits allocated.
func (bf *TargetBloomFilter) Capacity() uint64 {
	return bf.m
}

// Count returns the number of elements inserted.
func (bf *TargetBloomFilter) Count() uint64 {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	return bf.count
}

// DiscrepancyKind classifies differences between source and target observations.
type DiscrepancyKind string

const (
	DiscrepancyMissingInTarget DiscrepancyKind = "MISSING_IN_TARGET"
	DiscrepancyMissingInSource DiscrepancyKind = "MISSING_IN_SOURCE"
	DiscrepancyModified        DiscrepancyKind = "MODIFIED"
)

// ReconcileRecord represents an item for reconciliation comparison.
type ReconcileRecord struct {
	Key         string `json:"key"`
	Version     string `json:"version"`
	PayloadHash string `json:"payloadHash"`
}

// DiscrepancyItem describes a detected difference.
type DiscrepancyItem struct {
	Kind   DiscrepancyKind  `json:"kind"`
	Key    string           `json:"key"`
	Source *ReconcileRecord `json:"source,omitempty"`
	Target *ReconcileRecord `json:"target,omitempty"`
}

// ReconciliationSummary provides telemetry and diffs for a reconciliation run.
type ReconciliationSummary struct {
	SourceCount       int64             `json:"sourceCount"`
	TargetCount       int64             `json:"targetCount"`
	MatchedCount      int64             `json:"matchedCount"`
	DiscrepancyCount  int64             `json:"discrepancyCount"`
	BloomFilterSaves  int64             `json:"bloomFilterSaves"`
	Discrepancies     []DiscrepancyItem `json:"discrepancies,omitempty"`
	ExecutionDuration time.Duration     `json:"executionDuration"`
}

// ReconcileBatches performs two-phase reconciliation using Bloom pre-filtering.
func ReconcileBatches(
	sourceRecords []ReconcileRecord,
	targetRecords []ReconcileRecord,
	maxReportedDiscrepancies int,
) (*ReconciliationSummary, error) {
	if maxReportedDiscrepancies < 0 {
		return nil, errors.New("maxReportedDiscrepancies cannot be negative")
	}

	start := time.Now()
	summary := &ReconciliationSummary{
		SourceCount: int64(len(sourceRecords)),
		TargetCount: int64(len(targetRecords)),
	}

	// Index target records by key and populate target Bloom filter
	targetMap := make(map[string]ReconcileRecord, len(targetRecords))
	expectedCount := uint64(len(targetRecords))
	if expectedCount == 0 {
		expectedCount = 10
	}
	targetBloom, err := NewTargetBloomFilter(expectedCount, 0.01)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize target bloom filter: %w", err)
	}

	for _, tr := range targetRecords {
		targetMap[tr.Key] = tr
		targetBloom.Add(tr.Key)
	}

	// Track seen target keys
	matchedTargetKeys := make(map[string]struct{}, len(targetRecords))

	// Phase 1: Reconcile source against target
	for _, sr := range sourceRecords {
		// Fast-path: Bloom filter check
		if !targetBloom.Contains(sr.Key) {
			// Definite absence in target without map/db lookups
			summary.BloomFilterSaves++
			summary.DiscrepancyCount++
			if len(summary.Discrepancies) < maxReportedDiscrepancies {
				srcCopy := sr
				summary.Discrepancies = append(summary.Discrepancies, DiscrepancyItem{
					Kind:   DiscrepancyMissingInTarget,
					Key:    sr.Key,
					Source: &srcCopy,
				})
			}
			continue
		}

		// Bloom hit: verify with exact target map
		tr, exists := targetMap[sr.Key]
		if !exists {
			// False positive from Bloom filter
			summary.DiscrepancyCount++
			if len(summary.Discrepancies) < maxReportedDiscrepancies {
				srcCopy := sr
				summary.Discrepancies = append(summary.Discrepancies, DiscrepancyItem{
					Kind:   DiscrepancyMissingInTarget,
					Key:    sr.Key,
					Source: &srcCopy,
				})
			}
			continue
		}

		matchedTargetKeys[sr.Key] = struct{}{}
		if sr.Version != tr.Version || sr.PayloadHash != tr.PayloadHash {
			summary.DiscrepancyCount++
			if len(summary.Discrepancies) < maxReportedDiscrepancies {
				srcCopy := sr
				tgtCopy := tr
				summary.Discrepancies = append(summary.Discrepancies, DiscrepancyItem{
					Kind:   DiscrepancyModified,
					Key:    sr.Key,
					Source: &srcCopy,
					Target: &tgtCopy,
				})
			}
		} else {
			summary.MatchedCount++
		}
	}

	// Phase 2: Check for items missing in source
	for _, tr := range targetRecords {
		if _, seen := matchedTargetKeys[tr.Key]; !seen {
			summary.DiscrepancyCount++
			if len(summary.Discrepancies) < maxReportedDiscrepancies {
				tgtCopy := tr
				summary.Discrepancies = append(summary.Discrepancies, DiscrepancyItem{
					Kind:   DiscrepancyMissingInSource,
					Key:    tr.Key,
					Target: &tgtCopy,
				})
			}
		}
	}

	summary.ExecutionDuration = time.Since(start)
	return summary, nil
}
