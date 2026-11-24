package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"
)

// SnapshotItem represents a reconciled target state entity across regions.
type SnapshotItem struct {
	ID        string
	Version   int64
	Hash      string
	Payload   string
	Timestamp int64
}

// ComputeHash calculates a deterministic SHA-256 fingerprint for a snapshot item.
func (s *SnapshotItem) ComputeHash() string {
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%s:%d:%s", s.ID, s.Version, s.Payload)))
	return hex.EncodeToString(hasher.Sum(nil))
}

// DiffResult encapsulates divergence between two region snapshots.
type DiffResult struct {
	Added    []SnapshotItem
	Modified []SnapshotItem
	Deleted  []SnapshotItem
	Matched  int
}

// TotalDivergence returns the count of divergent items.
func (d DiffResult) TotalDivergence() int {
	return len(d.Added) + len(d.Modified) + len(d.Deleted)
}

// MerkleNode represents a node in the state snapshot hash tree.
type MerkleNode struct {
	Hash     string
	KeyRange [2]string // [minKey, maxKey]
	Left     *MerkleNode
	Right    *MerkleNode
	Items    []SnapshotItem // populated only on leaf nodes
	IsLeaf   bool
}

// ProfileResult encapsulates performance profiling metrics comparing diffing strategies.
type ProfileResult struct {
	TotalItems          int
	DivergenceCount     int
	Iterations          int
	LinearDurationTotal time.Duration
	LinearDurationMean  time.Duration
	LinearAllocBytes    uint64
	MerkleDurationTotal time.Duration
	MerkleDurationMean  time.Duration
	MerkleAllocBytes    uint64
	SpeedupFactor       float64
	P50DurationLinear   time.Duration
	P99DurationLinear   time.Duration
	P50DurationMerkle   time.Duration
	P99DurationMerkle   time.Duration
}

// SnapshotDiffProfiler benchmarks snapshot diffing and tree traversal strategies.
type SnapshotDiffProfiler struct {
	leafBucketCapacity int
}

// NewSnapshotDiffProfiler initializes a profiler with configurable leaf capacity.
func NewSnapshotDiffProfiler(leafBucketCapacity int) *SnapshotDiffProfiler {
	if leafBucketCapacity <= 0 {
		leafBucketCapacity = 64
	}
	return &SnapshotDiffProfiler{
		leafBucketCapacity: leafBucketCapacity,
	}
}

// DiffLinear executes a linear scan diff between source and target items.
func (p *SnapshotDiffProfiler) DiffLinear(source, target []SnapshotItem) DiffResult {
	sourceMap := make(map[string]SnapshotItem, len(source))
	for _, item := range source {
		sourceMap[item.ID] = item
	}

	targetMap := make(map[string]SnapshotItem, len(target))
	for _, item := range target {
		targetMap[item.ID] = item
	}

	var res DiffResult

	// Check items in target against source
	for id, tItem := range targetMap {
		sItem, exists := sourceMap[id]
		if !exists {
			res.Added = append(res.Added, tItem)
		} else if sItem.Hash != tItem.Hash || sItem.Version != tItem.Version {
			res.Modified = append(res.Modified, tItem)
		} else {
			res.Matched++
		}
	}

	// Check items deleted from source
	for id, sItem := range sourceMap {
		if _, exists := targetMap[id]; !exists {
			res.Deleted = append(res.Deleted, sItem)
		}
	}

	return res
}

// BuildMerkleTree constructs a hierarchical hash tree over snapshot items partitioned by ID prefix.
func (p *SnapshotDiffProfiler) BuildMerkleTree(items []SnapshotItem) *MerkleNode {
	if len(items) == 0 {
		return &MerkleNode{
			Hash:   "",
			IsLeaf: true,
		}
	}

	sorted := make([]SnapshotItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	return p.buildRecursive(sorted, 0)
}

func itemIDBit(id string, bitIndex int) int {
	h := sha256.Sum256([]byte(id))
	byteIdx := bitIndex / 8
	if byteIdx >= len(h) {
		return 0
	}
	bitOffset := 7 - (bitIndex % 8)
	return int((h[byteIdx] >> bitOffset) & 1)
}

func (p *SnapshotDiffProfiler) buildRecursive(items []SnapshotItem, bitDepth int) *MerkleNode {
	if len(items) <= p.leafBucketCapacity || bitDepth >= 64 {
		hasher := sha256.New()
		for _, it := range items {
			hasher.Write([]byte(it.ID + ":" + it.Hash))
		}
		var minKey, maxKey string
		if len(items) > 0 {
			minKey = items[0].ID
			maxKey = items[len(items)-1].ID
		}
		return &MerkleNode{
			Hash:     hex.EncodeToString(hasher.Sum(nil)),
			KeyRange: [2]string{minKey, maxKey},
			Items:    items,
			IsLeaf:   true,
		}
	}

	var leftItems, rightItems []SnapshotItem
	for _, it := range items {
		if itemIDBit(it.ID, bitDepth) == 0 {
			leftItems = append(leftItems, it)
		} else {
			rightItems = append(rightItems, it)
		}
	}

	// If all items fall on one side, increase depth to find split point
	if len(leftItems) == 0 || len(rightItems) == 0 {
		return p.buildRecursive(items, bitDepth+1)
	}

	left := p.buildRecursive(leftItems, bitDepth+1)
	right := p.buildRecursive(rightItems, bitDepth+1)

	parentHash := sha256.Sum256([]byte(left.Hash + ":" + right.Hash))
	return &MerkleNode{
		Hash:     hex.EncodeToString(parentHash[:]),
		KeyRange: [2]string{items[0].ID, items[len(items)-1].ID},
		Left:     left,
		Right:    right,
		IsLeaf:   false,
	}
}

// DiffMerkle compares two Merkle trees and isolates divergent sub-trees without traversing identical nodes.
func (p *SnapshotDiffProfiler) DiffMerkle(source, target *MerkleNode) DiffResult {
	var res DiffResult
	if source == nil || target == nil {
		return res
	}

	// Quick check: if root hashes match, trees are fully identical
	if source.Hash != "" && source.Hash == target.Hash {
		count := countItemsInTree(source)
		res.Matched = count
		return res
	}

	p.diffNodesRecursive(source, target, &res)
	return res
}

func countItemsInTree(node *MerkleNode) int {
	if node == nil {
		return 0
	}
	if node.IsLeaf {
		return len(node.Items)
	}
	return countItemsInTree(node.Left) + countItemsInTree(node.Right)
}

func (p *SnapshotDiffProfiler) diffNodesRecursive(source, target *MerkleNode, res *DiffResult) {
	if source == nil && target == nil {
		return
	}
	if source == nil {
		// All target items are added
		collectAllItems(target, &res.Added)
		return
	}
	if target == nil {
		// All source items are deleted
		collectAllItems(source, &res.Deleted)
		return
	}

	if source.Hash == target.Hash && source.Hash != "" {
		res.Matched += countItemsInTree(source)
		return
	}

	if source.IsLeaf || target.IsLeaf {
		var sItems, tItems []SnapshotItem
		collectAllItems(source, &sItems)
		collectAllItems(target, &tItems)
		subDiff := p.DiffLinear(sItems, tItems)
		res.Added = append(res.Added, subDiff.Added...)
		res.Modified = append(res.Modified, subDiff.Modified...)
		res.Deleted = append(res.Deleted, subDiff.Deleted...)
		res.Matched += subDiff.Matched
		return
	}

	p.diffNodesRecursive(source.Left, target.Left, res)
	p.diffNodesRecursive(source.Right, target.Right, res)
}

func collectAllItems(node *MerkleNode, dst *[]SnapshotItem) {
	if node == nil {
		return
	}
	if node.IsLeaf {
		*dst = append(*dst, node.Items...)
		return
	}
	collectAllItems(node.Left, dst)
	collectAllItems(node.Right, dst)
}

// RunBenchmark profiles linear vs Merkle tree diffing across multiple runs.
func (p *SnapshotDiffProfiler) RunBenchmark(source, target []SnapshotItem, iterations int) ProfileResult {
	if iterations <= 0 {
		iterations = 10
	}

	linearLatencies := make([]time.Duration, iterations)
	merkleLatencies := make([]time.Duration, iterations)

	sourceTree := p.BuildMerkleTree(source)
	targetTree := p.BuildMerkleTree(target)

	// Linear benchmark
	runtime.GC()
	var mBefore, mAfter runtime.MemStats
	runtime.ReadMemStats(&mBefore)

	var totalLinear time.Duration
	var lastLinearDiff DiffResult
	for i := 0; i < iterations; i++ {
		start := time.Now()
		for loop := 0; loop < 50; loop++ {
			lastLinearDiff = p.DiffLinear(source, target)
		}
		dur := time.Since(start)
		if dur <= 0 {
			dur = time.Microsecond
		}
		linearLatencies[i] = dur
		totalLinear += dur
	}
	runtime.ReadMemStats(&mAfter)
	linearAllocs := mAfter.TotalAlloc - mBefore.TotalAlloc

	// Merkle benchmark
	runtime.GC()
	runtime.ReadMemStats(&mBefore)

	var totalMerkle time.Duration
	for i := 0; i < iterations; i++ {
		start := time.Now()
		for loop := 0; loop < 50; loop++ {
			_ = p.DiffMerkle(sourceTree, targetTree)
		}
		dur := time.Since(start)
		if dur <= 0 {
			dur = time.Microsecond
		}
		merkleLatencies[i] = dur
		totalMerkle += dur
	}
	runtime.ReadMemStats(&mAfter)
	merkleAllocs := mAfter.TotalAlloc - mBefore.TotalAlloc

	sort.Slice(linearLatencies, func(i, j int) bool { return linearLatencies[i] < linearLatencies[j] })
	sort.Slice(merkleLatencies, func(i, j int) bool { return merkleLatencies[i] < merkleLatencies[j] })

	meanLinear := totalLinear / time.Duration(iterations)
	meanMerkle := totalMerkle / time.Duration(iterations)

	speedup := 1.0
	if meanMerkle > 0 {
		speedup = float64(meanLinear) / float64(meanMerkle)
	}

	p50Linear := linearLatencies[iterations/2]
	p99Linear := linearLatencies[int(float64(iterations)*0.99)]
	p50Merkle := merkleLatencies[iterations/2]
	p99Merkle := merkleLatencies[int(float64(iterations)*0.99)]

	return ProfileResult{
		TotalItems:          len(source),
		DivergenceCount:     lastLinearDiff.TotalDivergence(),
		Iterations:          iterations,
		LinearDurationTotal: totalLinear,
		LinearDurationMean:  meanLinear,
		LinearAllocBytes:    linearAllocs,
		MerkleDurationTotal: totalMerkle,
		MerkleDurationMean:  meanMerkle,
		MerkleAllocBytes:    merkleAllocs,
		SpeedupFactor:       speedup,
		P50DurationLinear:   p50Linear,
		P99DurationLinear:   p99Linear,
		P50DurationMerkle:   p50Merkle,
		P99DurationMerkle:   p99Merkle,
	}
}

// FormatBenchmarkMarkdown generates a structured performance report table.
func (p *SnapshotDiffProfiler) FormatBenchmarkMarkdown(res ProfileResult) string {
	var b strings.Builder
	b.WriteString("### Multi-Region Snapshot Diffing & Hash Tree Benchmark Summary\n\n")
	b.WriteString(fmt.Sprintf("- **Total State Items:** `%d`\n", res.TotalItems))
	b.WriteString(fmt.Sprintf("- **Divergent Items:** `%d`\n", res.DivergenceCount))
	b.WriteString(fmt.Sprintf("- **Iterations Evaluated:** `%d`\n", res.Iterations))
	b.WriteString(fmt.Sprintf("- **Speedup Factor (Linear / Merkle):** `%.2fx`\n\n", res.SpeedupFactor))

	b.WriteString("| Metric | Linear O(N) Scan | Merkle Hash Tree | Improvement |\n")
	b.WriteString("|---|---|---|---|\n")
	b.WriteString(fmt.Sprintf("| **Mean Latency** | `%v` | `%v` | `%.2fx` |\n", res.LinearDurationMean, res.MerkleDurationMean, res.SpeedupFactor))
	b.WriteString(fmt.Sprintf("| **P50 Latency** | `%v` | `%v` | - |\n", res.P50DurationLinear, res.P50DurationMerkle))
	b.WriteString(fmt.Sprintf("| **P99 Latency** | `%v` | `%v` | - |\n", res.P99DurationLinear, res.P99DurationMerkle))
	b.WriteString(fmt.Sprintf("| **Total Allocs (Bytes)** | `%d` | `%d` | - |\n", res.LinearAllocBytes, res.MerkleAllocBytes))

	return b.String()
}
