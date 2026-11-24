package core

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestReconciliationPlanContract_ValidNominalPlan(t *testing.T) {
	contract := NewReconciliationPlanContract(DefaultPlanSafetyLimits())

	plan := DiffPlan{
		PlanID: "plan-reconcile-2026.11-east",
		Diffs: []ResourceDiff{
			{ResourceID: "service/frontend", Action: "create", Details: "deploy replica 3"},
			{ResourceID: "service/backend", Action: "UPDATE", Details: "bump image v2.1"},
			{ResourceID: "service/cache", Action: "noop", Details: "synced"},
			{ResourceID: "service/legacy-api", Action: "DELETE", Details: "deprecated"},
		},
	}

	sanitized, summary, err := contract.ValidateAndSanitize(plan)
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	if sanitized.PlanID != "plan-reconcile-2026.11-east" {
		t.Fatalf("unexpected sanitized plan ID: %s", sanitized.PlanID)
	}
	if len(sanitized.Diffs) != 4 {
		t.Fatalf("expected 4 diffs, got %d", len(sanitized.Diffs))
	}
	// Action normalized to uppercase
	if sanitized.Diffs[0].Action != "CREATE" || sanitized.Diffs[2].Action != "NOOP" {
		t.Fatalf("expected normalized uppercase actions, got %s and %s", sanitized.Diffs[0].Action, sanitized.Diffs[2].Action)
	}
	if !summary.HasDestructiveOps {
		t.Fatalf("expected HasDestructiveOps to be true due to DELETE")
	}
}

func TestReconciliationPlanContract_InvalidPlanID(t *testing.T) {
	contract := NewReconciliationPlanContract(DefaultPlanSafetyLimits())

	invalidIDs := []string{"", "  ", "ab", "plan with spaces", "plan/with/slashes", "plan;inject"}
	for _, id := range invalidIDs {
		plan := DiffPlan{PlanID: id, Diffs: []ResourceDiff{{ResourceID: "res-1", Action: "CREATE"}}}
		_, _, err := contract.ValidateAndSanitize(plan)
		if !errors.Is(err, ErrInvalidPlanID) {
			t.Fatalf("expected ErrInvalidPlanID for '%s', got %v", id, err)
		}
	}
}

func TestReconciliationPlanContract_DuplicateResourceDiff(t *testing.T) {
	contract := NewReconciliationPlanContract(DefaultPlanSafetyLimits())

	plan := DiffPlan{
		PlanID: "plan-duplicate-check",
		Diffs: []ResourceDiff{
			{ResourceID: "app/worker-pool", Action: "CREATE"},
			{ResourceID: "app/worker-pool", Action: "DELETE"},
		},
	}

	_, _, err := contract.ValidateAndSanitize(plan)
	if !errors.Is(err, ErrDuplicateResourceDiff) {
		t.Fatalf("expected ErrDuplicateResourceDiff, got %v", err)
	}
}

func TestReconciliationPlanContract_UnsupportedAction(t *testing.T) {
	contract := NewReconciliationPlanContract(DefaultPlanSafetyLimits())

	plan := DiffPlan{
		PlanID: "plan-unsupported-action",
		Diffs: []ResourceDiff{
			{ResourceID: "db/primary", Action: "DROP_DATABASE"},
		},
	}

	_, _, err := contract.ValidateAndSanitize(plan)
	if !errors.Is(err, ErrUnsupportedAction) {
		t.Fatalf("expected ErrUnsupportedAction, got %v", err)
	}
}

func TestReconciliationPlanContract_BlastRadiusLimits(t *testing.T) {
	limits := DefaultPlanSafetyLimits()
	limits.MaxDeletionsPerPlan = 3
	limits.MaxDeletionRatio = 0.30
	contract := NewReconciliationPlanContract(limits)

	// Exceeds count limit: 4 deletions > 3
	diffsOverCount := make([]ResourceDiff, 10)
	for i := 0; i < 4; i++ {
		diffsOverCount[i] = ResourceDiff{ResourceID: fmt.Sprintf("res-del-%d", i), Action: "DELETE"}
	}
	for i := 4; i < 10; i++ {
		diffsOverCount[i] = ResourceDiff{ResourceID: fmt.Sprintf("res-up-%d", i), Action: "UPDATE"}
	}

	planOverCount := DiffPlan{PlanID: "plan-over-count", Diffs: diffsOverCount}
	_, _, err := contract.ValidateAndSanitize(planOverCount)
	if !errors.Is(err, ErrBlastRadiusExceeded) {
		t.Fatalf("expected ErrBlastRadiusExceeded for deletion count, got %v", err)
	}

	// Exceeds ratio limit: 3 deletions out of 6 active resources (50% > 30%)
	limits.MaxDeletionsPerPlan = 10
	contractRatio := NewReconciliationPlanContract(limits)

	diffsOverRatio := []ResourceDiff{
		{ResourceID: "r1", Action: "DELETE"},
		{ResourceID: "r2", Action: "DELETE"},
		{ResourceID: "r3", Action: "DELETE"},
		{ResourceID: "r4", Action: "CREATE"},
		{ResourceID: "r5", Action: "CREATE"},
		{ResourceID: "r6", Action: "UPDATE"},
	}
	planOverRatio := DiffPlan{PlanID: "plan-over-ratio", Diffs: diffsOverRatio}
	_, _, err = contractRatio.ValidateAndSanitize(planOverRatio)
	if !errors.Is(err, ErrBlastRadiusExceeded) {
		t.Fatalf("expected ErrBlastRadiusExceeded for deletion ratio, got %v", err)
	}
}

func TestReconciliationPlanContract_DetailsTruncation(t *testing.T) {
	limits := DefaultPlanSafetyLimits()
	limits.MaxDetailsLength = 10
	contract := NewReconciliationPlanContract(limits)

	plan := DiffPlan{
		PlanID: "plan-truncation",
		Diffs: []ResourceDiff{
			{ResourceID: "res-long", Action: "UPDATE", Details: strings.Repeat("A", 100)},
		},
	}

	sanitized, _, err := contract.ValidateAndSanitize(plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sanitized.Diffs[0].Details) != 10 {
		t.Fatalf("expected details truncated to 10 chars, got %d", len(sanitized.Diffs[0].Details))
	}
}

func TestReconciliationPlanContract_Concurrency(t *testing.T) {
	contract := NewReconciliationPlanContract(DefaultPlanSafetyLimits())

	var wg sync.WaitGroup
	workers := 8
	iterations := 50

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				p := DiffPlan{
					PlanID: fmt.Sprintf("plan-worker-%d-%d", id, i),
					Diffs: []ResourceDiff{
						{ResourceID: fmt.Sprintf("res-%d-%d-a", id, i), Action: "CREATE"},
						{ResourceID: fmt.Sprintf("res-%d-%d-b", id, i), Action: "UPDATE"},
					},
				}
				_, _, err := contract.ValidateAndSanitize(p)
				if err != nil {
					t.Errorf("worker %d failed: %v", id, err)
				}
			}
		}(w)
	}

	wg.Wait()
}
