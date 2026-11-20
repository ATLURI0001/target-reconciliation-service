package core

import "testing"

func TestDiffPlan(t *testing.T) {
	plan := DiffPlan{
		PlanID: "plan-001",
		Diffs: []ResourceDiff{
			{ResourceID: "r1", Action: "CREATE"},
			{ResourceID: "r2", Action: "UPDATE"},
			{ResourceID: "r3", Action: "NOOP"},
			{ResourceID: "r4", Action: "CREATE"},
		},
	}

	counts := plan.ActionCounts()
	if counts["CREATE"] != 2 {
		t.Fatalf("expected 2 CREATE actions, got %d", counts["CREATE"])
	}
	if counts["UPDATE"] != 1 {
		t.Fatalf("expected 1 UPDATE action, got %d", counts["UPDATE"])
	}
	if counts["NOOP"] != 1 {
		t.Fatalf("expected 1 NOOP action, got %d", counts["NOOP"])
	}

	if !plan.NeedsAction() {
		t.Fatal("expected NeedsAction to return true when CREATE/UPDATE diffs exist")
	}

	// All NOOP plan
	noopPlan := DiffPlan{
		PlanID: "plan-002",
		Diffs: []ResourceDiff{
			{ResourceID: "r1", Action: "NOOP"},
			{ResourceID: "r2", Action: "NOOP"},
		},
	}
	if noopPlan.NeedsAction() {
		t.Fatal("expected NeedsAction to return false for all-NOOP plan")
	}
}
