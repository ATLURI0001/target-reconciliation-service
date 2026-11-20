package core

import (
	"testing"
)

func TestDetectStateDrift_CleanState(t *testing.T) {
	desired := map[string]*ResourceSpec{
		"vpc-1": {ID: "vpc-1", PayloadHash: "hash-aaa", Version: 1},
		"db-1":  {ID: "db-1", PayloadHash: "hash-bbb", Version: 2},
	}
	actual := map[string]*ResourceSpec{
		"vpc-1": {ID: "vpc-1", PayloadHash: "hash-aaa", Version: 1},
		"db-1":  {ID: "db-1", PayloadHash: "hash-bbb", Version: 2},
	}

	report, err := DetectStateDrift(desired, actual)
	if err != nil {
		t.Fatalf("unexpected drift error: %v", err)
	}

	if report.DriftedCount != 0 {
		t.Fatalf("expected 0 drifts, got %d", report.DriftedCount)
	}
	if report.Severity != "CLEAN" {
		t.Fatalf("expected CLEAN severity, got %s", report.Severity)
	}
	if report.DriftRatio != 0.0 {
		t.Fatalf("expected drift ratio 0.0, got %f", report.DriftRatio)
	}
}

func TestDetectStateDrift_MutatedAndOrphanAndMissing(t *testing.T) {
	desired := map[string]*ResourceSpec{
		"vpc-1":     {ID: "vpc-1", PayloadHash: "hash-aaa", Version: 1},
		"missing-1": {ID: "missing-1", PayloadHash: "hash-mmm", Version: 1},
		"mutated-1": {ID: "mutated-1", PayloadHash: "hash-desired", Version: 2},
	}
	actual := map[string]*ResourceSpec{
		"vpc-1":     {ID: "vpc-1", PayloadHash: "hash-aaa", Version: 1},
		"mutated-1": {ID: "mutated-1", PayloadHash: "hash-actual-tampered", Version: 1},
		"orphan-1":  {ID: "orphan-1", PayloadHash: "hash-untracked", Version: 5},
	}

	report, err := DetectStateDrift(desired, actual)
	if err != nil {
		t.Fatalf("unexpected drift error: %v", err)
	}

	if report.DriftedCount != 3 {
		t.Fatalf("expected 3 drifted resources, got %d", report.DriftedCount)
	}

	typesFound := make(map[DriftType]bool)
	for _, d := range report.Drifts {
		typesFound[d.Type] = true
	}

	if !typesFound[DriftTypeMissing] {
		t.Fatal("expected to find DriftTypeMissing")
	}
	if !typesFound[DriftTypeMutated] {
		t.Fatal("expected to find DriftTypeMutated")
	}
	if !typesFound[DriftTypeOrphan] {
		t.Fatal("expected to find DriftTypeOrphan")
	}

	if report.Severity != "CRITICAL" {
		t.Fatalf("expected CRITICAL severity, got %s", report.Severity)
	}
}

func TestGenerateCompensatingRollback(t *testing.T) {
	executed := []ResourceDiff{
		{ResourceID: "res-create", Action: "CREATE"},
		{ResourceID: "res-update", Action: "UPDATE"},
		{ResourceID: "res-delete", Action: "DELETE"},
	}

	baseSnapshots := map[string]*ResourceSpec{
		"res-update": {ID: "res-update", PayloadHash: "hash-base-update"},
		"res-delete": {ID: "res-delete", PayloadHash: "hash-base-delete"},
	}

	rollback, err := GenerateCompensatingRollback(executed, baseSnapshots)
	if err != nil {
		t.Fatalf("unexpected rollback error: %v", err)
	}

	// Should be inverted (LIFO order: res-delete, res-update, res-create)
	if len(rollback) != 3 {
		t.Fatalf("expected 3 rollback steps, got %d", len(rollback))
	}

	// Step 0: undo DELETE -> ROLLBACK_RESTORE
	if rollback[0].ResourceID != "res-delete" || rollback[0].Action != "ROLLBACK_RESTORE" || rollback[0].TargetHash != "hash-base-delete" {
		t.Fatalf("unexpected rollback step 0: %+v", rollback[0])
	}

	// Step 1: undo UPDATE -> ROLLBACK_RESTORE
	if rollback[1].ResourceID != "res-update" || rollback[1].Action != "ROLLBACK_RESTORE" || rollback[1].TargetHash != "hash-base-update" {
		t.Fatalf("unexpected rollback step 1: %+v", rollback[1])
	}

	// Step 2: undo CREATE -> ROLLBACK_DELETE
	if rollback[2].ResourceID != "res-create" || rollback[2].Action != "ROLLBACK_DELETE" {
		t.Fatalf("unexpected rollback step 2: %+v", rollback[2])
	}

	// Failure case: missing base snapshot for UPDATE
	_, errMissing := GenerateCompensatingRollback(
		[]ResourceDiff{{ResourceID: "unknown", Action: "UPDATE"}},
		map[string]*ResourceSpec{},
	)
	if errMissing == nil {
		t.Fatal("expected error when base snapshot is missing for UPDATE rollback")
	}
}
