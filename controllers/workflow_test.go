package controllers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ATLURI0001/resilienceplane/internal/core"
	"github.com/ATLURI0001/resilienceplane/internal/store"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// This is a cross-component test with an in-memory durable-store contract and a
// fake Kubernetes client. Status changes below are explicit test inputs; they
// are not measured rollout health or evidence of a real Kubernetes execution.
func TestWorkflowDetectionApprovalRollbackVerificationAndAudit(t *testing.T) {
	ctx := context.Background()
	repository := store.NewMemory()
	svc := core.NewService(repository)
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	svc.Now = func() time.Time { return now }
	operator := core.Identity{Subject: "test-operator", Role: "operator"}
	if err := svc.Register(ctx, operator, core.Cluster{ID: "local", Environment: "test"}); err != nil {
		t.Fatal(err)
	}
	p := core.DefaultPolicy()
	p.DetectionSeconds = 1
	p.DryRun = false
	if err := svc.SetPolicy(ctx, operator, p); err != nil {
		t.Fatal(err)
	}
	d := sampleDeployment()
	target := Target(DemoNamespace, "catalog-health", "catalog", "catalog", goodImage)
	c := fake.NewClientBuilder().WithScheme(scheme(t)).WithStatusSubresource(&appsv1.Deployment{}).WithObjects(d, target).Build()
	reconciler := &TargetReconciler{Client: c, Workflow: svc, ClusterID: "local", Now: func() time.Time { return now }}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: DemoNamespace, Name: "catalog-health"}}
	reconcile := func() {
		t.Helper()
		if _, err := reconciler.Reconcile(ctx, req); err != nil {
			t.Fatal(err)
		}
	}
	reconcile()
	now = now.Add(2 * time.Second)
	reconcile()
	state, err := repository.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Incidents) != 1 {
		t.Fatalf("expected one incident, got %d", len(state.Incidents))
	}
	var incident core.Incident
	for _, i := range state.Incidents {
		incident = i
	}
	if incident.State != "AwaitingApproval" {
		t.Fatalf("expected approval gate, got %+v", incident)
	}
	if err := svc.Approve(ctx, core.Identity{Subject: "reader", Role: "viewer"}, incident.ID); !errors.Is(err, core.ErrDenied) {
		t.Fatalf("expected viewer denial, got %v", err)
	}
	reconcile()
	var live appsv1.Deployment
	if err := c.Get(ctx, client.ObjectKeyFromObject(d), &live); err != nil {
		t.Fatal(err)
	}
	if live.Spec.Template.Spec.Containers[0].Image == goodImage {
		t.Fatal("unauthorized approval mutated deployment")
	}
	if err := svc.Approve(ctx, operator, incident.ID); err != nil {
		t.Fatal(err)
	}
	// A management process restart reuses persisted workflow state.
	restarted := core.NewService(repository)
	restarted.Now = func() time.Time { return now }
	reconciler.Workflow = restarted
	reconcile()
	state, err = repository.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Incidents[incident.ID].State != "Verifying" {
		t.Fatalf("expected Verifying, got %+v", state.Incidents[incident.ID])
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(d), &live); err != nil {
		t.Fatal(err)
	}
	if live.Spec.Template.Spec.Containers[0].Image != goodImage || live.Spec.Template.Annotations[ExecutionAnnotation] == "" {
		t.Fatal("approved rollback absent")
	}
	// The fake client has no Deployment controller. Supply the fresh API status
	// that a real Deployment controller must establish before resolution.
	live.Status.Replicas = 1
	live.Status.ReadyReplicas = 1
	live.Status.AvailableReplicas = 1
	live.Status.UpdatedReplicas = 1
	live.Status.ObservedGeneration = live.Generation
	if err := c.Status().Update(ctx, &live); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	reconcile()
	state, err = repository.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Incidents[incident.ID].State != "Resolved" {
		t.Fatalf("expected verified resolution, got %+v", state.Incidents[incident.ID])
	}
	actions := map[string]bool{}
	denial := false
	for _, a := range state.Audit {
		actions[a.Action] = true
		if a.Actor == "reader" && a.Result == "denied" {
			denial = true
		}
	}
	for _, action := range []string{"incident.detect", "plan.evaluate", "incident.execute", "incident.verify"} {
		if !actions[action] {
			t.Fatalf("missing audit %s: %+v", action, state.Audit)
		}
	}
	if !denial {
		t.Fatal("authorization denial not audited")
	}
}
