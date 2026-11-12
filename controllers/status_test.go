package controllers

import (
	"context"
	"errors"
	"github.com/ATLURI0001/target-reconciliation-service/internal/core"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func condition(t *testing.T, target *unstructured.Unstructured, kind string) metav1.Condition {
	t.Helper()
	raw, _, err := unstructured.NestedSlice(target.Object, "status", "conditions")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range raw {
		var condition metav1.Condition
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.(map[string]any), &condition); err != nil {
			t.Fatal(err)
		}
		if condition.Type == kind {
			return condition
		}
	}
	t.Fatalf("missing %s condition: %+v", kind, target.Object)
	return metav1.Condition{}
}
func readTarget(t *testing.T, c client.Client, key client.ObjectKey) *unstructured.Unstructured {
	t.Helper()
	target := &unstructured.Unstructured{}
	target.SetGroupVersionKind(TargetGVK)
	if err := c.Get(context.Background(), key, target); err != nil {
		t.Fatal(err)
	}
	return target
}
func healthyStatus(d *appsv1.Deployment) {
	d.Status.Replicas = 1
	d.Status.AvailableReplicas = 1
	d.Status.ReadyReplicas = 1
	d.Status.UpdatedReplicas = 1
	d.Status.ObservedGeneration = d.Generation
}

func TestStatusHealthTransitionsNoopAndGeneration(t *testing.T) {
	ctx := context.Background()
	d := sampleDeployment()
	healthyStatus(d)
	target := Target(DemoNamespace, "status-target", d.Name, "catalog", goodImage)
	target.SetGeneration(1)
	c := fake.NewClientBuilder().WithScheme(scheme(t)).WithStatusSubresource(target, &appsv1.Deployment{}).WithObjects(d, target).Build()
	now := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	r := &TargetReconciler{Client: c, Workflow: &workflowStub{}, ClusterID: "local", Now: func() time.Time { return now }}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(target)}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	first := readTarget(t, c, req.NamespacedName)
	ready := condition(t, first, "Ready")
	if ready.Status != metav1.ConditionTrue || ready.Reason != "DeploymentReady" || ready.ObservedGeneration != 1 {
		t.Fatalf("ready=%+v", ready)
	}
	rv := first.GetResourceVersion()
	now = now.Add(time.Minute)
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	unchanged := readTarget(t, c, req.NamespacedName)
	if unchanged.GetResourceVersion() != rv || !condition(t, unchanged, "Ready").LastTransitionTime.Time.Equal(ready.LastTransitionTime.Time) {
		t.Fatal("unchanged status was rewritten")
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(d), d); err != nil {
		t.Fatal(err)
	}
	d.Status.AvailableReplicas = 0
	d.Status.ReadyReplicas = 0
	if err := c.Status().Update(ctx, d); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	changed := readTarget(t, c, req.NamespacedName)
	unready := condition(t, changed, "Ready")
	if unready.Status != metav1.ConditionFalse || !unready.LastTransitionTime.Time.Equal(now) {
		t.Fatalf("unready=%+v", unready)
	}
	firstReconciled := conditionTime(first, "Reconciled")
	if !condition(t, changed, "Reconciled").LastTransitionTime.Time.Equal(firstReconciled.Time) {
		t.Fatal("Reconciled timestamp changed despite stable True")
	}
	changed.SetGeneration(2)
	if err := c.Update(ctx, changed); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	latest := readTarget(t, c, req.NamespacedName)
	latestReady := condition(t, latest, "Ready")
	observed, _, _ := unstructured.NestedInt64(latest.Object, "status", "observedGeneration")
	if observed != 2 || latestReady.ObservedGeneration != 2 || !latestReady.LastTransitionTime.Time.Equal(unready.LastTransitionTime.Time) {
		t.Fatalf("observed=%d condition=%+v", observed, latestReady)
	}
}
func conditionTime(target *unstructured.Unstructured, kind string) metav1.Time {
	raw, _, _ := unstructured.NestedSlice(target.Object, "status", "conditions")
	for _, item := range raw {
		var c metav1.Condition
		_ = runtime.DefaultUnstructuredConverter.FromUnstructured(item.(map[string]any), &c)
		if c.Type == kind {
			return c.LastTransitionTime
		}
	}
	return metav1.Time{}
}

func TestStatusUnknownOnUnavailablePreservesEvidence(t *testing.T) {
	ctx := context.Background()
	d := sampleDeployment()
	healthyStatus(d)
	target := Target(DemoNamespace, "unavailable-target", d.Name, "catalog", goodImage)
	target.SetGeneration(1)
	c := fake.NewClientBuilder().WithScheme(scheme(t)).WithStatusSubresource(target).WithObjects(d, target).Build()
	w := &workflowStub{}
	r := &TargetReconciler{Client: c, Workflow: w, ClusterID: "local"}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(target)}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	previous := readTarget(t, c, req.NamespacedName)
	evidence, _, _ := unstructured.NestedMap(previous.Object, "status", "lastObservation")
	r.Reader = unavailableDeploymentReader{Reader: c}
	calls := w.claims
	if _, err := r.Reconcile(ctx, req); err == nil || !strings.Contains(err.Error(), "live Kubernetes API unavailable") {
		t.Fatalf("unexpected error %v", err)
	}
	latest := readTarget(t, c, req.NamespacedName)
	if ready := condition(t, latest, "Ready"); ready.Status != metav1.ConditionUnknown || ready.Reason != "ObservationUnavailable" {
		t.Fatalf("Ready=%+v", ready)
	}
	last, _, _ := unstructured.NestedMap(latest.Object, "status", "lastObservation")
	if last["deploymentUID"] != evidence["deploymentUID"] || w.claims != calls {
		t.Fatal("unknown observation replaced evidence or claimed work")
	}
}

func TestStatusSpecInvalidAndRollingSurgeNotReady(t *testing.T) {
	ctx := context.Background()
	d := sampleDeployment()
	healthyStatus(d)
	d.Status.Replicas = 2
	target := Target(DemoNamespace, "surge-target", d.Name, "catalog", goodImage)
	target.SetGeneration(1)
	c := fake.NewClientBuilder().WithScheme(scheme(t)).WithStatusSubresource(target).WithObjects(d, target).Build()
	r := &TargetReconciler{Client: c, Workflow: &workflowStub{}, ClusterID: "local"}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(target)}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if ready := condition(t, readTarget(t, c, req.NamespacedName), "Ready"); ready.Status != metav1.ConditionFalse {
		t.Fatalf("surge overhang falsely ready: %+v", ready)
	}
	broken := readTarget(t, c, req.NamespacedName)
	_ = unstructured.SetNestedField(broken.Object, "nginx:latest", "spec", "healthyImage")
	if err := c.Update(ctx, broken); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, req); err == nil {
		t.Fatal("invalid spec accepted")
	}
	if ready := condition(t, readTarget(t, c, req.NamespacedName), "Ready"); ready.Status != metav1.ConditionUnknown || ready.Reason != "SpecInvalid" {
		t.Fatalf("invalid condition %+v", ready)
	}
}

type statusClient struct {
	client.Client
	attempts     int
	failure      error
	conflictOnce bool
}

func (c *statusClient) Status() client.SubResourceWriter {
	return &statusWriter{SubResourceWriter: c.Client.Status(), owner: c}
}

type statusWriter struct {
	client.SubResourceWriter
	owner *statusClient
}

func (w *statusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, options ...client.SubResourcePatchOption) error {
	w.owner.attempts++
	if w.owner.conflictOnce && w.owner.attempts == 1 {
		return apierrors.NewConflict(schema.GroupResource{Group: TargetGVK.Group, Resource: "reliabilitytargets"}, obj.GetName(), errors.New("concurrent update"))
	}
	if w.owner.failure != nil {
		return w.owner.failure
	}
	return w.SubResourceWriter.Patch(ctx, obj, patch, options...)
}
func TestStatusConflictRetryAndOriginalErrorPreserved(t *testing.T) {
	target := Target(DemoNamespace, "conflict-target", "catalog", "catalog", goodImage)
	target.SetGeneration(1)
	base := fake.NewClientBuilder().WithScheme(scheme(t)).WithStatusSubresource(target).WithObjects(target).Build()
	c := &statusClient{Client: base, conflictOnce: true}
	r := &TargetReconciler{Client: c}
	if err := r.publishStatus(context.Background(), target, unknownStatus("ObservationUnavailable", "unavailable")); err != nil {
		t.Fatal(err)
	}
	if c.attempts != 2 {
		t.Fatalf("attempts=%d", c.attempts)
	}
	c.failure = errors.New("status writer failed")
	original := errors.New("original workflow failure")
	_, err := r.finish(context.Background(), target, unknownStatus("WorkflowFailed", "workflow error"), original)
	if !errors.Is(err, original) || !errors.Is(err, c.failure) {
		t.Fatalf("original/status errors lost: %v", err)
	}
}
func TestStatusGenerationFenceAndWatchPredicate(t *testing.T) {
	target := Target(DemoNamespace, "fence-target", "catalog", "catalog", goodImage)
	target.SetGeneration(1)
	newer := target.DeepCopy()
	newer.SetGeneration(2)
	c := fake.NewClientBuilder().WithScheme(scheme(t)).WithStatusSubresource(target).WithObjects(newer).Build()
	r := &TargetReconciler{Client: c}
	if err := r.publishStatus(context.Background(), target, unknownStatus("ObservationUnavailable", "old evidence")); err == nil {
		t.Fatal("stale CR generation published")
	}
	statusOnly := target.DeepCopy()
	statusOnly.Object["status"] = map[string]any{"observedGeneration": int64(1)}
	filter := predicate.GenerationChangedPredicate{}
	if filter.Update(event.UpdateEvent{ObjectOld: target, ObjectNew: statusOnly}) {
		t.Fatal("status-only event passed generation filter")
	}
	if !filter.Update(event.UpdateEvent{ObjectOld: target, ObjectNew: newer}) {
		t.Fatal("spec-generation update filtered")
	}
}

type observeFailureWorkflow struct {
	workflowStub
	err error
}

func (w *observeFailureWorkflow) Observe(context.Context, core.Observation) error { return w.err }
func TestStatusWorkflowFailure(t *testing.T) {
	d := sampleDeployment()
	healthyStatus(d)
	target := Target(DemoNamespace, "workflow-failure-target", d.Name, "catalog", goodImage)
	c := fake.NewClientBuilder().WithScheme(scheme(t)).WithStatusSubresource(target).WithObjects(d, target).Build()
	wanted := errors.New("storage unavailable")
	workflow := &observeFailureWorkflow{err: wanted}
	r := &TargetReconciler{Client: c, Workflow: workflow, ClusterID: "local"}
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(target)})
	if !errors.Is(err, wanted) {
		t.Fatalf("workflow error lost: %v", err)
	}
	result := readTarget(t, c, client.ObjectKeyFromObject(target))
	if condition(t, result, "Ready").Status != metav1.ConditionUnknown || condition(t, result, "Reconciled").Reason != "WorkflowFailed" || workflow.claims != 0 {
		t.Fatalf("workflow failure status invalid: %+v", result.Object)
	}
}
