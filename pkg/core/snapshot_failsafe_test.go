package core

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSnapshotFailsafe_CreateSnapshotAndHash(t *testing.T) {
	sentinel := NewSnapshotFailsafeSentinel(DefaultSnapshotFailsafeConfig())

	resources := map[string]ResourceRecord{
		"res-vpc-01": {
			ID:         "res-vpc-01",
			Kind:       "VPC",
			Version:    1,
			SpecHash:   "abc123hash",
			Attributes: map[string]string{"cidr": "10.0.0.0/16"},
			UpdatedAt:  time.Now(),
		},
		"res-sub-01": {
			ID:         "res-sub-01",
			Kind:       "Subnet",
			Version:    1,
			SpecHash:   "def456hash",
			Attributes: map[string]string{"cidr": "10.0.1.0/24"},
			UpdatedAt:  time.Now(),
		},
	}

	snap, err := sentinel.CreateSnapshot(resources)
	if err != nil {
		t.Fatalf("unexpected error creating snapshot: %v", err)
	}

	if snap.TotalItems != 2 {
		t.Errorf("expected 2 items, got %d", snap.TotalItems)
	}
	if len(snap.StateHash) != 64 {
		t.Errorf("expected 64-char SHA-256 state hash, got %s", snap.StateHash)
	}

	// Corrupt resource rejection
	badResources := map[string]ResourceRecord{
		"res-bad": {ID: "", Kind: "VPC"},
	}
	_, err = sentinel.CreateSnapshot(badResources)
	if !errors.Is(err, ErrCorruptResourceState) {
		t.Errorf("expected ErrCorruptResourceState, got %v", err)
	}
}

func TestSnapshotFailsafe_SuccessfulCommit(t *testing.T) {
	sentinel := NewSnapshotFailsafeSentinel(DefaultSnapshotFailsafeConfig())

	initial := map[string]ResourceRecord{
		"app-server-1": {ID: "app-server-1", Kind: "ComputeInstance", Version: 1},
	}

	result, rb, err := sentinel.ExecuteWithFailsafe(initial, func(candidate map[string]ResourceRecord) error {
		candidate["app-server-2"] = ResourceRecord{ID: "app-server-2", Kind: "ComputeInstance", Version: 1}
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error during commit: %v", err)
	}
	if rb != nil {
		t.Fatalf("expected nil rollback record on success, got: %v", rb)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 resources in committed state, got %d", len(result))
	}

	telemetry := sentinel.GetTelemetry()
	if telemetry.TotalTransactions != 1 || telemetry.CommittedCount != 1 || telemetry.RollbackCount != 0 {
		t.Errorf("unexpected telemetry stats: %+v", telemetry)
	}
}

func TestSnapshotFailsafe_RollbackOnError(t *testing.T) {
	sentinel := NewSnapshotFailsafeSentinel(DefaultSnapshotFailsafeConfig())

	initial := map[string]ResourceRecord{
		"database-primary": {ID: "database-primary", Kind: "PostgresDB", Version: 1},
	}

	expectedErr := errors.New("cloud provider rate limit reached")
	result, rb, err := sentinel.ExecuteWithFailsafe(initial, func(candidate map[string]ResourceRecord) error {
		// Poison candidate state before failing
		delete(candidate, "database-primary")
		candidate["rogue-db"] = ResourceRecord{ID: "rogue-db", Kind: "PostgresDB", Version: 999}
		return expectedErr
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if rb == nil {
		t.Fatal("expected rollback record, got nil")
	}
	if !strings.Contains(rb.TriggerReason, "cloud provider rate limit reached") {
		t.Errorf("unexpected trigger reason: %s", rb.TriggerReason)
	}

	// Verify pristine state restored
	if len(result) != 1 || result["database-primary"].ID != "database-primary" {
		t.Errorf("state was not restored to initial snapshot: %+v", result)
	}

	telemetry := sentinel.GetTelemetry()
	if telemetry.RollbackCount != 1 || telemetry.CommittedCount != 0 {
		t.Errorf("expected 1 rollback, got %+v", telemetry)
	}
}

func TestSnapshotFailsafe_RollbackOnPanic(t *testing.T) {
	sentinel := NewSnapshotFailsafeSentinel(DefaultSnapshotFailsafeConfig())

	initial := map[string]ResourceRecord{
		"k8s-cluster": {ID: "k8s-cluster", Kind: "KubernetesCluster", Version: 1},
	}

	result, rb, err := sentinel.ExecuteWithFailsafe(initial, func(candidate map[string]ResourceRecord) error {
		delete(candidate, "k8s-cluster")
		panic("nil pointer dereference inside cloud SDK")
	})

	if err == nil {
		t.Fatal("expected error on panic rollback")
	}
	if rb == nil {
		t.Fatal("expected rollback record on panic")
	}
	if !strings.Contains(rb.TriggerReason, "nil pointer dereference") {
		t.Errorf("unexpected panic trigger reason: %s", rb.TriggerReason)
	}
	if len(result) != 1 || result["k8s-cluster"].ID != "k8s-cluster" {
		t.Errorf("failed to restore initial state after panic: %+v", result)
	}
}

func TestSnapshotFailsafe_DestructionThresholdBreach(t *testing.T) {
	cfg := DefaultSnapshotFailsafeConfig()
	cfg.MaxDestructionThreshold = 2
	sentinel := NewSnapshotFailsafeSentinel(cfg)

	initial := map[string]ResourceRecord{
		"r1": {ID: "r1", Kind: "Resource", Version: 1},
		"r2": {ID: "r2", Kind: "Resource", Version: 1},
		"r3": {ID: "r3", Kind: "Resource", Version: 1},
		"r4": {ID: "r4", Kind: "Resource", Version: 1},
	}

	// Attempt to delete 3 resources (exceeding threshold of 2)
	result, rb, err := sentinel.ExecuteWithFailsafe(initial, func(candidate map[string]ResourceRecord) error {
		delete(candidate, "r1")
		delete(candidate, "r2")
		delete(candidate, "r3")
		return nil
	})

	if !errors.Is(err, ErrDestructionExceeded) {
		t.Errorf("expected ErrDestructionExceeded, got %v", err)
	}
	if rb == nil {
		t.Fatal("expected rollback record on destruction threshold breach")
	}
	if len(result) != 4 {
		t.Errorf("expected all 4 resources restored, got %d", len(result))
	}
}
