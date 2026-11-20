package core

import (
	"errors"
	"testing"
)

func TestThreeWayDiff(t *testing.T) {
	specBase := &ResourceSpec{ID: "db-01", PayloadHash: "hash-v1"}
	specActual := &ResourceSpec{ID: "db-01", PayloadHash: "hash-v1"}
	specDesired := &ResourceSpec{ID: "db-01", PayloadHash: "hash-v2"}

	// Standard UPDATE: desired changed, actual remained at base
	diff, err := ThreeWayDiff(specBase, specActual, specDesired)
	if err != nil || diff.Action != "UPDATE" {
		t.Fatalf("expected UPDATE action, got: %s (err: %v)", diff.Action, err)
	}

	// Converged NOOP
	diffNoop, err := ThreeWayDiff(specBase, specDesired, specDesired)
	if err != nil || diffNoop.Action != "NOOP" {
		t.Fatalf("expected NOOP action, got: %s", diffNoop.Action)
	}

	// CREATE: actual is nil
	diffCreate, err := ThreeWayDiff(nil, nil, specDesired)
	if err != nil || diffCreate.Action != "CREATE" {
		t.Fatalf("expected CREATE action, got: %s", diffCreate.Action)
	}

	// DELETE: desired is nil
	diffDelete, err := ThreeWayDiff(specBase, specActual, nil)
	if err != nil || diffDelete.Action != "DELETE" {
		t.Fatalf("expected DELETE action, got: %s", diffDelete.Action)
	}

	// CONFLICT: both actual and desired diverged from base
	specActualDiverged := &ResourceSpec{ID: "db-01", PayloadHash: "hash-v1-manual"}
	diffConflict, err := ThreeWayDiff(specBase, specActualDiverged, specDesired)
	if err != nil || diffConflict.Action != "CONFLICT" {
		t.Fatalf("expected CONFLICT action, got: %s", diffConflict.Action)
	}
}

func TestPlanExecutionDAG_TopologicalStages(t *testing.T) {
	// Diamond DAG:
	// db -> app1, app2
	// app1, app2 -> gateway
	specs := []ResourceSpec{
		{ID: "db", DependsOn: nil},
		{ID: "app1", DependsOn: []string{"db"}},
		{ID: "app2", DependsOn: []string{"db"}},
		{ID: "gateway", DependsOn: []string{"app1", "app2"}},
	}

	plan, err := PlanExecutionDAG(specs)
	if err != nil {
		t.Fatalf("unexpected error planning DAG: %v", err)
	}

	if len(plan.Stages) != 3 {
		t.Fatalf("expected 3 stages (db -> [app1, app2] -> gateway), got %d", len(plan.Stages))
	}

	// Stage 0: ["db"]
	if len(plan.Stages[0].ResourceIDs) != 1 || plan.Stages[0].ResourceIDs[0] != "db" {
		t.Fatalf("expected stage 0 to contain ['db'], got: %v", plan.Stages[0].ResourceIDs)
	}

	// Stage 1: ["app1", "app2"]
	if len(plan.Stages[1].ResourceIDs) != 2 || plan.Stages[1].ResourceIDs[0] != "app1" || plan.Stages[1].ResourceIDs[1] != "app2" {
		t.Fatalf("expected stage 1 to contain ['app1', 'app2'], got: %v", plan.Stages[1].ResourceIDs)
	}

	// Stage 2: ["gateway"]
	if len(plan.Stages[2].ResourceIDs) != 1 || plan.Stages[2].ResourceIDs[0] != "gateway" {
		t.Fatalf("expected stage 2 to contain ['gateway'], got: %v", plan.Stages[2].ResourceIDs)
	}
}

func TestPlanExecutionDAG_CycleDetection(t *testing.T) {
	// Cycle: A -> B -> C -> A
	specs := []ResourceSpec{
		{ID: "A", DependsOn: []string{"C"}},
		{ID: "B", DependsOn: []string{"A"}},
		{ID: "C", DependsOn: []string{"B"}},
	}

	_, err := PlanExecutionDAG(specs)
	if !errors.Is(err, ErrCyclicDependency) {
		t.Fatalf("expected ErrCyclicDependency, got: %v", err)
	}
}

func TestPlanExecutionDAG_MissingAndDuplicateValidation(t *testing.T) {
	// Missing dependency
	missingSpecs := []ResourceSpec{
		{ID: "worker", DependsOn: []string{"ghost-service"}},
	}
	_, err := PlanExecutionDAG(missingSpecs)
	if !errors.Is(err, ErrMissingDependency) {
		t.Fatalf("expected ErrMissingDependency, got: %v", err)
	}

	// Duplicate ID
	dupSpecs := []ResourceSpec{
		{ID: "svc", DependsOn: nil},
		{ID: "svc", DependsOn: nil},
	}
	_, err = PlanExecutionDAG(dupSpecs)
	if !errors.Is(err, ErrDuplicateResource) {
		t.Fatalf("expected ErrDuplicateResource, got: %v", err)
	}
}
