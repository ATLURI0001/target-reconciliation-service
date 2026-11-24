package core

import (
	"fmt"
	"strings"
	"testing"
)

func generateTestItems(count int, prefix string) []SnapshotItem {
	items := make([]SnapshotItem, count)
	for i := 0; i < count; i++ {
		item := SnapshotItem{
			ID:        fmt.Sprintf("%s-target-%04d", prefix, i),
			Version:   int64(1),
			Payload:   fmt.Sprintf("config-payload-%d", i),
			Timestamp: 1000 + int64(i),
		}
		item.Hash = item.ComputeHash()
		items[i] = item
	}
	return items
}

func TestSnapshotItem_ComputeHash(t *testing.T) {
	it1 := SnapshotItem{ID: "t-1", Version: 1, Payload: "data"}
	it2 := SnapshotItem{ID: "t-1", Version: 1, Payload: "data"}
	it3 := SnapshotItem{ID: "t-1", Version: 2, Payload: "data"}

	if it1.ComputeHash() != it2.ComputeHash() {
		t.Fatalf("expected identical hashes for identical items")
	}
	if it1.ComputeHash() == it3.ComputeHash() {
		t.Fatalf("expected different hashes for different versions")
	}
}

func TestSnapshotDiffProfiler_DiffLinear(t *testing.T) {
	profiler := NewSnapshotDiffProfiler(4)

	source := generateTestItems(10, "us-east")
	target := generateTestItems(10, "us-east")

	// Target modifications
	target[2].Version = 2
	target[2].Hash = target[2].ComputeHash() // modified

	// Target additions & deletions
	target = append(target, SnapshotItem{ID: "new-target", Version: 1, Payload: "new", Hash: "h-new"})
	source = append(source, SnapshotItem{ID: "deleted-target", Version: 1, Payload: "del", Hash: "h-del"})

	diff := profiler.DiffLinear(source, target)

	if len(diff.Added) != 1 || diff.Added[0].ID != "new-target" {
		t.Fatalf("expected 1 added item, got %d", len(diff.Added))
	}
	if len(diff.Modified) != 1 || diff.Modified[0].ID != "us-east-target-0002" {
		t.Fatalf("expected 1 modified item, got %d", len(diff.Modified))
	}
	if len(diff.Deleted) != 1 || diff.Deleted[0].ID != "deleted-target" {
		t.Fatalf("expected 1 deleted item, got %d", len(diff.Deleted))
	}
	if diff.Matched != 9 {
		t.Fatalf("expected 9 matched items, got %d", diff.Matched)
	}
}

func TestSnapshotDiffProfiler_MerkleDiffMatchesLinear(t *testing.T) {
	profiler := NewSnapshotDiffProfiler(4)

	source := generateTestItems(50, "region-a")
	target := generateTestItems(50, "region-a")

	// Mutate some elements
	target[5].Version = 3
	target[5].Hash = target[5].ComputeHash()
	target[25].Payload = "altered"
	target[25].Hash = target[25].ComputeHash()

	// Append additions & deletions
	target = append(target, SnapshotItem{ID: "extra-item-1", Version: 1, Hash: "hash1"})
	target = append(target, SnapshotItem{ID: "extra-item-2", Version: 1, Hash: "hash2"})
	source = append(source, SnapshotItem{ID: "orphan-item-1", Version: 1, Hash: "hash3"})

	linearDiff := profiler.DiffLinear(source, target)

	sTree := profiler.BuildMerkleTree(source)
	tTree := profiler.BuildMerkleTree(target)
	merkleDiff := profiler.DiffMerkle(sTree, tTree)

	if linearDiff.TotalDivergence() != merkleDiff.TotalDivergence() {
		t.Fatalf("expected equal divergence count: linear=%d, merkle=%d",
			linearDiff.TotalDivergence(), merkleDiff.TotalDivergence())
	}
	if len(linearDiff.Added) != len(merkleDiff.Added) {
		t.Fatalf("added mismatch: linear=%d, merkle=%d", len(linearDiff.Added), len(merkleDiff.Added))
	}
	if len(linearDiff.Modified) != len(merkleDiff.Modified) {
		t.Fatalf("modified mismatch: linear=%d, merkle=%d", len(linearDiff.Modified), len(merkleDiff.Modified))
	}
	if len(linearDiff.Deleted) != len(merkleDiff.Deleted) {
		t.Fatalf("deleted mismatch: linear=%d, merkle=%d", len(linearDiff.Deleted), len(merkleDiff.Deleted))
	}
}

func TestSnapshotDiffProfiler_IdenticalTreesShortCircuit(t *testing.T) {
	profiler := NewSnapshotDiffProfiler(8)
	source := generateTestItems(40, "cluster-x")

	sTree := profiler.BuildMerkleTree(source)
	tTree := profiler.BuildMerkleTree(source)

	diff := profiler.DiffMerkle(sTree, tTree)
	if diff.TotalDivergence() != 0 {
		t.Fatalf("expected 0 divergence for identical trees, got %d", diff.TotalDivergence())
	}
	if diff.Matched != 40 {
		t.Fatalf("expected 40 matched items, got %d", diff.Matched)
	}
}

func TestSnapshotDiffProfiler_EmptyTreeHandling(t *testing.T) {
	profiler := NewSnapshotDiffProfiler(4)

	emptyTree := profiler.BuildMerkleTree(nil)
	if emptyTree.Hash != "" || !emptyTree.IsLeaf {
		t.Fatalf("expected empty leaf node for nil items")
	}

	items := generateTestItems(5, "node")
	tree := profiler.BuildMerkleTree(items)

	diffTargetEmpty := profiler.DiffMerkle(tree, nil)
	if diffTargetEmpty.TotalDivergence() != 0 {
		t.Fatalf("expected 0 for nil tree")
	}

	diffNilSource := profiler.DiffMerkle(nil, tree)
	if diffNilSource.TotalDivergence() != 0 {
		t.Fatalf("expected 0 for nil tree")
	}
}

func TestSnapshotDiffProfiler_RunBenchmarkAndFormat(t *testing.T) {
	profiler := NewSnapshotDiffProfiler(8)

	source := generateTestItems(100, "region-prod")
	target := generateTestItems(100, "region-prod")
	target[10].Payload = "mutation"
	target[10].Hash = target[10].ComputeHash()

	res := profiler.RunBenchmark(source, target, 5)

	if res.TotalItems != 100 {
		t.Fatalf("expected 100 items, got %d", res.TotalItems)
	}
	if res.DivergenceCount != 1 {
		t.Fatalf("expected 1 divergence, got %d", res.DivergenceCount)
	}
	if res.Iterations != 5 {
		t.Fatalf("expected 5 iterations, got %d", res.Iterations)
	}
	if res.LinearDurationTotal <= 0 || res.MerkleDurationTotal <= 0 {
		t.Fatalf("expected positive benchmark durations")
	}

	markdown := profiler.FormatBenchmarkMarkdown(res)
	if !strings.Contains(markdown, "Multi-Region Snapshot Diffing & Hash Tree Benchmark Summary") {
		t.Fatalf("expected summary header in markdown")
	}
	if !strings.Contains(markdown, "Linear O(N) Scan") {
		t.Fatalf("expected linear scan column in table")
	}
}
