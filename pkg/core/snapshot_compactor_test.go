package core

import (
	"sync"
	"testing"
	"time"
)

func TestSnapshotCompactor_AppendAndMonotonicSequence(t *testing.T) {
	compactor := NewSnapshotCompactor(DefaultCompactorConfig())

	// Sequence 1
	err := compactor.AppendDelta(StateDelta{
		Sequence:   1,
		ResourceID: "db/primary",
		Op:         DeltaCreate,
		Payload:    map[string]any{"engine": "postgres", "replicas": 2},
	})
	if err != nil {
		t.Fatalf("unexpected error appending sequence 1: %v", err)
	}

	// Sequence 2
	err = compactor.AppendDelta(StateDelta{
		Sequence:   2,
		ResourceID: "db/cache",
		Op:         DeltaCreate,
		Payload:    map[string]any{"engine": "redis", "memory_mb": 1024},
	})
	if err != nil {
		t.Fatalf("unexpected error appending sequence 2: %v", err)
	}

	// Attempt non-monotonic sequence (e.g. sequence 4 skipping 3)
	err = compactor.AppendDelta(StateDelta{
		Sequence:   4,
		ResourceID: "db/analytics",
		Op:         DeltaCreate,
		Payload:    map[string]any{"engine": "clickhouse"},
	})
	if err == nil {
		t.Fatal("expected error on out-of-order sequence gap, got nil")
	}

	// Attempt empty resource ID
	err = compactor.AppendDelta(StateDelta{
		Sequence: 3,
		Op:       DeltaCreate,
	})
	if err != ErrInvalidDelta {
		t.Fatalf("expected ErrInvalidDelta, got %v", err)
	}
}

func TestSnapshotCompactor_CompactionAndStateTransitions(t *testing.T) {
	cfg := CompactorConfig{
		CompactionThreshold: 3,
		RetentionSnapshots:  2,
	}
	compactor := NewSnapshotCompactor(cfg)

	// Delta 1: CREATE res1
	_ = compactor.AppendDelta(StateDelta{
		Sequence:   1,
		ResourceID: "res1",
		Op:         DeltaCreate,
		Payload:    map[string]any{"status": "INITIAL", "tier": "standard"},
	})

	// Delta 2: PATCH res1
	_ = compactor.AppendDelta(StateDelta{
		Sequence:   2,
		ResourceID: "res1",
		Op:         DeltaPatch,
		Payload:    map[string]any{"status": "ACTIVE", "region": "us-east-1"},
	})

	// Delta 3: CREATE res2
	_ = compactor.AppendDelta(StateDelta{
		Sequence:   3,
		ResourceID: "res2",
		Op:         DeltaCreate,
		Payload:    map[string]any{"allocated": 100},
	})

	if !compactor.ShouldCompact() {
		t.Fatalf("expected ShouldCompact=true at threshold 3, got false")
	}

	snap, err := compactor.Compact()
	if err != nil {
		t.Fatalf("compaction failed: %v", err)
	}

	if snap.BaseSequence != 3 {
		t.Errorf("expected BaseSequence 3, got %d", snap.BaseSequence)
	}
	if snap.ResourceCount != 2 {
		t.Errorf("expected 2 resources, got %d", snap.ResourceCount)
	}

	// Check merged state of res1
	res1 := snap.Resources["res1"]
	if res1["status"] != "ACTIVE" || res1["tier"] != "standard" || res1["region"] != "us-east-1" {
		t.Errorf("unexpected merged state for res1: %+v", res1)
	}

	// Verify pending deltas were cleared
	if compactor.PendingDeltasCount() != 0 {
		t.Errorf("expected 0 pending deltas, got %d", compactor.PendingDeltasCount())
	}

	// Verify hash integrity
	if !compactor.VerifySnapshotHash(snap) {
		t.Errorf("snapshot hash verification failed")
	}
}

func TestSnapshotCompactor_DeletionAndReplay(t *testing.T) {
	compactor := NewSnapshotCompactor(DefaultCompactorConfig())

	// Sequence 1: CREATE r1
	_ = compactor.AppendDelta(StateDelta{
		Sequence:   1,
		ResourceID: "r1",
		Op:         DeltaCreate,
		Payload:    map[string]any{"v": 1},
	})
	// Sequence 2: CREATE r2
	_ = compactor.AppendDelta(StateDelta{
		Sequence:   2,
		ResourceID: "r2",
		Op:         DeltaCreate,
		Payload:    map[string]any{"v": 2},
	})

	snap1, err := compactor.Compact()
	if err != nil {
		t.Fatalf("compact failed: %v", err)
	}

	// Delta 3: DELETE r1
	// Delta 4: UPDATE r2
	deltas := []StateDelta{
		{Sequence: 3, ResourceID: "r1", Op: DeltaDelete, Timestamp: time.Now()},
		{Sequence: 4, ResourceID: "r2", Op: DeltaUpdate, Payload: map[string]any{"v": 99}, Timestamp: time.Now()},
	}

	replayed, err := compactor.ReplayFromSnapshot(snap1, deltas)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	if _, exists := replayed["r1"]; exists {
		t.Errorf("expected r1 to be deleted in replayed state")
	}

	if replayed["r2"]["v"] != 99 {
		t.Errorf("expected r2 v=99, got %v", replayed["r2"]["v"])
	}

	// Tampered snapshot hash test
	snap1.SnapshotHash = "tampered-hash-deadbeef"
	_, err = compactor.ReplayFromSnapshot(snap1, deltas)
	if err != ErrSnapshotIntegrityViolation {
		t.Fatalf("expected ErrSnapshotIntegrityViolation, got %v", err)
	}
}

func TestSnapshotCompactor_SnapshotRetention(t *testing.T) {
	cfg := CompactorConfig{
		CompactionThreshold: 1,
		RetentionSnapshots:  2,
	}
	compactor := NewSnapshotCompactor(cfg)

	for seq := uint64(1); seq <= 5; seq++ {
		_ = compactor.AppendDelta(StateDelta{
			Sequence:   seq,
			ResourceID: "node",
			Op:         DeltaUpdate,
			Payload:    map[string]any{"tick": seq},
		})
		_, _ = compactor.Compact()
	}

	compactor.mu.RLock()
	defer compactor.mu.RUnlock()
	if len(compactor.snapshots) > 2 {
		t.Errorf("retention exceeded: expected at most 2 snapshots, got %d", len(compactor.snapshots))
	}
}

func TestSnapshotCompactor_ConcurrentAppendAndRead(t *testing.T) {
	compactor := NewSnapshotCompactor(DefaultCompactorConfig())
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				err := compactor.AppendDelta(StateDelta{
					Sequence:   0, // auto monotonic sequence
					ResourceID: "worker-node",
					Op:         DeltaPatch,
					Payload:    map[string]any{"worker": workerID, "step": j},
				})
				if err != nil {
					t.Errorf("unexpected error on concurrent append: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	if compactor.PendingDeltasCount() != 100 {
		t.Errorf("expected 100 pending deltas, got %d", compactor.PendingDeltasCount())
	}

	snap, err := compactor.Compact()
	if err != nil {
		t.Fatalf("concurrent compaction failed: %v", err)
	}

	if snap.BaseSequence != 100 {
		t.Errorf("expected BaseSequence 100, got %d", snap.BaseSequence)
	}
}
