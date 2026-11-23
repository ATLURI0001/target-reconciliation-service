package core

import (
	"testing"
)

func TestReconciliationHashGuard_ValidPlanAndDigest(t *testing.T) {
	guard := NewReconciliationHashGuard(DefaultHashGuardConfig())

	res1 := ReconciliationResource{
		ResourceID: "service/api-gateway",
		Kind:       "Deployment",
		Version:    1,
		Attributes: map[string]string{"replicas": "3", "image": "v1.2.0"},
	}
	res1.StateHash = ComputeDeterministicStateHash(res1)

	plan := []ResourcePlanItem{
		{Action: ActionCreate, Resource: res1},
	}

	if err := guard.ValidateReconciliationPlan(10, plan, ""); err != nil {
		t.Fatalf("expected valid plan, got: %v", err)
	}

	// Update to version 2
	res1v2 := res1
	res1v2.Version = 2
	res1v2.Attributes["replicas"] = "5"
	res1v2.StateHash = ComputeDeterministicStateHash(res1v2)

	planUpdate := []ResourcePlanItem{
		{Action: ActionUpdate, Resource: res1v2},
	}
	if err := guard.ValidateReconciliationPlan(10, planUpdate, ""); err != nil {
		t.Fatalf("expected valid update plan, got: %v", err)
	}
}

func TestReconciliationHashGuard_ProtectedResourceViolation(t *testing.T) {
	guard := NewReconciliationHashGuard(DefaultHashGuardConfig())

	sysRes := ReconciliationResource{
		ResourceID: "kube-system/core-dns",
		Kind:       "DaemonSet",
		Version:    1,
	}

	deletePlan := []ResourcePlanItem{
		{Action: ActionDelete, Resource: sysRes},
	}

	err := guard.ValidateReconciliationPlan(10, deletePlan, "")
	if err == nil {
		t.Fatalf("expected error modifying protected resource, got nil")
	}
}

func TestReconciliationHashGuard_BlastRadiusCeiling(t *testing.T) {
	cfg := DefaultHashGuardConfig()
	cfg.MaxDestructionThresholdPct = 25.0
	cfg.MinResourcesForBlastRadius = 4
	guard := NewReconciliationHashGuard(cfg)

	// Attempt to delete 3 out of 10 resources (30% > 25%)
	deletePlan := []ResourcePlanItem{
		{Action: ActionDelete, Resource: ReconciliationResource{ResourceID: "app/worker-1"}},
		{Action: ActionDelete, Resource: ReconciliationResource{ResourceID: "app/worker-2"}},
		{Action: ActionDelete, Resource: ReconciliationResource{ResourceID: "app/worker-3"}},
	}

	err := guard.ValidateReconciliationPlan(10, deletePlan, "")
	if err == nil {
		t.Fatalf("expected blast radius exceeded error, got nil")
	}
}

func TestReconciliationHashGuard_TamperedStateDigest(t *testing.T) {
	guard := NewReconciliationHashGuard(DefaultHashGuardConfig())

	res := ReconciliationResource{
		ResourceID: "db/primary-cluster",
		Kind:       "StatefulSet",
		Version:    1,
		StateHash:  "forged-unmatched-hash-value-12345678",
	}

	plan := []ResourcePlanItem{
		{Action: ActionCreate, Resource: res},
	}

	err := guard.ValidateReconciliationPlan(10, plan, "")
	if err == nil {
		t.Fatalf("expected digest mismatch error, got nil")
	}
}

func TestReconciliationHashGuard_MonotonicVersionRollback(t *testing.T) {
	guard := NewReconciliationHashGuard(DefaultHashGuardConfig())

	resV5 := ReconciliationResource{
		ResourceID: "cache/redis",
		Kind:       "Service",
		Version:    5,
	}
	resV5.StateHash = ComputeDeterministicStateHash(resV5)
	if err := guard.ValidateReconciliationPlan(10, []ResourcePlanItem{{Action: ActionCreate, Resource: resV5}}, ""); err != nil {
		t.Fatalf("setup error: %v", err)
	}

	// Stale rollback attempt with version 4
	resV4 := resV5
	resV4.Version = 4
	resV4.StateHash = ComputeDeterministicStateHash(resV4)

	err := guard.ValidateReconciliationPlan(10, []ResourcePlanItem{{Action: ActionUpdate, Resource: resV4}}, "")
	if err == nil {
		t.Fatalf("expected monotonic version rollback error, got nil")
	}
}
