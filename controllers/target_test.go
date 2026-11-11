package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ATLURI0001/resilienceplane/internal/core"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const goodImage = "registry.example/catalog@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func sampleDeployment() *appsv1.Deployment {
	one := int32(1)
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "catalog", Namespace: DemoNamespace, UID: "uid-1", ResourceVersion: "1", Generation: 1, Labels: map[string]string{OptInLabel: "true"}}, Spec: appsv1.DeploymentSpec{Replicas: &one, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "catalog"}}, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "catalog"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "catalog", Image: "registry.example/catalog:broken"}}}}}}
}
func scheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := appsv1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	return s
}

type workflowStub struct {
	observations []core.Observation
	claims       int
	incident     *core.Incident
	verifying    bool
	failure      string
}

func (w *workflowStub) Observe(_ context.Context, o core.Observation) error {
	w.observations = append(w.observations, o)
	return nil
}
func (w *workflowStub) Claim(context.Context, string) (*core.Incident, error) {
	w.claims++
	return w.incident, nil
}
func (w *workflowStub) MarkVerifying(context.Context, string) error      { w.verifying = true; return nil }
func (w *workflowStub) Fail(_ context.Context, _ string, s string) error { w.failure = s; return nil }
func plan(r *TargetReconciler, d *appsv1.Deployment) core.Incident {
	o, _ := r.observe(d, "catalog", goodImage)
	return core.Incident{ID: "i-1", State: "Executing", ExecutionKey: "exec-1", Plan: core.Plan{Action: "rollback", Target: o, Image: goodImage}}
}
func TestReconcileReadsTargetDeploymentAndPassesObservation(t *testing.T) {
	d := sampleDeployment()
	target := Target(DemoNamespace, "separate-target-name", "catalog", "catalog", goodImage)
	c := fake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(d, target).Build()
	w := &workflowStub{}
	r := &TargetReconciler{Client: c, Workflow: w, ClusterID: "local", Now: func() time.Time { return time.Unix(100, 0) }}
	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: DemoNamespace, Name: target.GetName()}})
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != 5*time.Second || len(w.observations) != 1 {
		t.Fatalf("result=%v observations=%v", result, w.observations)
	}
	if o := w.observations[0]; o.Name != "catalog" || o.Healthy() || !o.OptIn || o.At.Unix() != 100 {
		t.Fatalf("bad observation: %+v", o)
	}
}
func TestRollbackFakeClientAndIdempotentReplay(t *testing.T) {
	ctx := context.Background()
	d := sampleDeployment()
	c := fake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(d).Build()
	r := &TargetReconciler{Client: c, ClusterID: "local"}
	p := plan(r, d)
	if err := r.Execute(ctx, d, p); err != nil {
		t.Fatal(err)
	}
	var live appsv1.Deployment
	if err := c.Get(ctx, client.ObjectKeyFromObject(d), &live); err != nil {
		t.Fatal(err)
	}
	if live.Spec.Template.Spec.Containers[0].Image != goodImage || live.Spec.Template.Annotations[ExecutionAnnotation] != p.ExecutionKey {
		t.Fatal("rollback did not persist image and execution marker")
	}
	rv := live.ResourceVersion
	if err := r.Execute(ctx, &live, p); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(d), &live); err != nil {
		t.Fatal(err)
	}
	if live.ResourceVersion != rv {
		t.Fatal("replay mutated resource")
	}
}
func TestExecutionGuards(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*appsv1.Deployment, *core.Incident)
	}{
		{"uid", func(d *appsv1.Deployment, _ *core.Incident) { d.UID = "replacement" }},
		{"namespace", func(d *appsv1.Deployment, _ *core.Incident) { d.Namespace = "kube-system" }},
		{"optin", func(d *appsv1.Deployment, _ *core.Incident) { delete(d.Labels, OptInLabel) }},
		{"gitops", func(d *appsv1.Deployment, _ *core.Incident) { d.Labels["argocd.argoproj.io/instance"] = "application" }},
		{"flux", func(d *appsv1.Deployment, _ *core.Incident) {
			d.Labels["kustomize.toolkit.fluxcd.io/name"] = "application"
		}},
		{"image drift", func(d *appsv1.Deployment, _ *core.Incident) { d.Spec.Template.Spec.Containers[0].Image = "other" }},
		{"unpinned image", func(_ *appsv1.Deployment, p *core.Incident) { p.Plan.Image = "nginx:latest" }},
		{"generation drift", func(d *appsv1.Deployment, _ *core.Incident) { d.Generation++ }},
		{"replica drift", func(d *appsv1.Deployment, _ *core.Incident) { *d.Spec.Replicas = 2 }},
		{"not executing", func(_ *appsv1.Deployment, p *core.Incident) { p.State = "AwaitingApproval" }},
		{"no key", func(_ *appsv1.Deployment, p *core.Incident) { p.ExecutionKey = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := sampleDeployment()
			c := &patchRecorder{}
			r := &TargetReconciler{Client: c, ClusterID: "local"}
			p := plan(r, d)
			tc.mutate(d, &p)
			if err := r.Execute(context.Background(), d, p); err == nil {
				t.Fatal("expected denial")
			}
			if c.called {
				t.Fatal("denied action attempted patch")
			}
		})
	}
}

type patchRecorder struct {
	client.Client
	called  bool
	payload []byte
	failure error
}

func (p *patchRecorder) Patch(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
	p.called = true
	b, err := patch.Data(o)
	p.payload = b
	if err != nil {
		return err
	}
	return p.failure
}
func TestPatchHasAtomicPreconditions(t *testing.T) {
	d := sampleDeployment()
	c := &patchRecorder{}
	r := &TargetReconciler{Client: c, ClusterID: "local"}
	p := plan(r, d)
	if err := r.Execute(context.Background(), d, p); err != nil {
		t.Fatal(err)
	}
	var ops []patchOperation
	if err := json.Unmarshal(c.payload, &ops); err != nil {
		t.Fatal(err)
	}
	if len(ops) < 5 || ops[0].Op != "test" || ops[0].Path != "/metadata/uid" || ops[1].Path != "/metadata/resourceVersion" || ops[2].Op != "test" {
		t.Fatalf("missing atomic tests: %s", c.payload)
	}
}
func TestUnknownDeploymentNeverReportsHealthy(t *testing.T) {
	target := Target(DemoNamespace, "target", "missing", "catalog", goodImage)
	c := fake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(target).Build()
	w := &workflowStub{}
	r := &TargetReconciler{Client: c, Workflow: w, ClusterID: "local"}
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: DemoNamespace, Name: "target"}})
	if err == nil || len(w.observations) != 0 || w.verifying {
		t.Fatal("unobservable target reported as healthy")
	}
}
func TestInvalidTargetRejected(t *testing.T) {
	target := Target(DemoNamespace, "target", "catalog", "catalog", "nginx:latest")
	c := fake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(target).Build()
	w := &workflowStub{}
	r := &TargetReconciler{Client: c, Workflow: w}
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: DemoNamespace, Name: "target"}})
	if err == nil || !strings.Contains(err.Error(), "digest-pinned") {
		t.Fatalf("unexpected error %v", err)
	}
}
func TestPatchErrorPropagates(t *testing.T) {
	d := sampleDeployment()
	wanted := errors.New("API unreachable")
	c := &patchRecorder{failure: wanted}
	r := &TargetReconciler{Client: c, ClusterID: "local"}
	if err := r.Execute(context.Background(), d, plan(r, d)); !errors.Is(err, wanted) {
		t.Fatal(fmt.Sprintf("got %v", err))
	}
}

type unavailableDeploymentReader struct{ client.Reader }

func (r unavailableDeploymentReader) Get(ctx context.Context, key client.ObjectKey, object client.Object, options ...client.GetOption) error {
	if _, ok := object.(*appsv1.Deployment); ok {
		return errors.New("live Kubernetes API unavailable")
	}
	return r.Reader.Get(ctx, key, object, options...)
}

func TestUnavailableLiveReaderNeverRefreshesCachedObservation(t *testing.T) {
	// The cached client still contains a Deployment, but the configured live
	// APIReader cannot reach Kubernetes. No heartbeat or execution claim may be
	// produced from that cached snapshot.
	d := sampleDeployment()
	target := Target(DemoNamespace, "target", "catalog", "catalog", goodImage)
	cached := fake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(d, target).Build()
	workflow := &workflowStub{}
	reconciler := &TargetReconciler{Client: cached, Reader: unavailableDeploymentReader{Reader: cached}, Workflow: workflow, ClusterID: "local"}
	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: DemoNamespace, Name: "target"}})
	if err == nil || !strings.Contains(err.Error(), "live Kubernetes API unavailable") {
		t.Fatalf("expected live-read failure, got %v", err)
	}
	if len(workflow.observations) != 0 || workflow.claims != 0 || workflow.verifying {
		t.Fatalf("cached evidence reached workflow: %+v", workflow)
	}
}
