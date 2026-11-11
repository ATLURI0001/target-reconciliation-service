// Package controllers implements the local, namespace-scoped Kubernetes bridge.
// It does not distribute kubeconfigs and is not an outbound multi-cluster agent.
package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ATLURI0001/resilienceplane/internal/core"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DemoNamespace       = "rp-demo"
	ExecutionAnnotation = "resilienceplane.io/execution-key"
	OptInLabel          = "resilienceplane.io/allow-remediation"
)

var TargetGVK = schema.GroupVersionKind{Group: "reliability.resilienceplane.io", Version: "v1alpha1", Kind: "ReliabilityTarget"}
var ErrUnsafePlan = errors.New("unsafe execution plan")
var digestImage = regexp.MustCompile(`^\S+@sha256:[a-f0-9]{64}$`)

type Workflow interface {
	Observe(context.Context, core.Observation) error
	Claim(context.Context, string) (*core.Incident, error)
	MarkVerifying(context.Context, string) error
	Fail(context.Context, string, string) error
}

type TargetReconciler struct {
	Client    client.Client
	Reader    client.Reader
	Workflow  Workflow
	ClusterID string
	Now       func() time.Time
}

// Setup registers a genuine controller-runtime reconciler. The manager must use
// a namespace-scoped cache and enable leader election outside single-process dev.
func Setup(mgr ctrl.Manager, workflow Workflow, _ core.Store, clusterID string) error {
	target := &unstructured.Unstructured{}
	target.SetGroupVersionKind(TargetGVK)
	return ctrl.NewControllerManagedBy(mgr).Named("reliability-target").For(target).Complete(&TargetReconciler{Client: mgr.GetClient(), Reader: mgr.GetAPIReader(), Workflow: workflow, ClusterID: clusterID, Now: time.Now})
}

func (r *TargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Namespace != DemoNamespace {
		return ctrl.Result{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	target := &unstructured.Unstructured{}
	target.SetGroupVersionKind(TargetGVK)
	reader := r.Reader
	if reader == nil {
		reader = r.Client
	}
	if err := reader.Get(ctx, req.NamespacedName, target); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	name, _, _ := unstructured.NestedString(target.Object, "spec", "deployment")
	container, _, _ := unstructured.NestedString(target.Object, "spec", "container")
	image, _, _ := unstructured.NestedString(target.Object, "spec", "healthyImage")
	if name == "" || container == "" || !digestImage.MatchString(image) {
		return ctrl.Result{}, fmt.Errorf("invalid ReliabilityTarget: deployment, container and digest-pinned healthyImage required")
	}
	var deployment appsv1.Deployment
	if err := reader.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: name}, &deployment); err != nil {
		return ctrl.Result{}, err
	}
	observation, err := r.observe(&deployment, container, image)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err = r.Workflow.Observe(ctx, observation); err != nil {
		return ctrl.Result{}, err
	}
	incident, err := r.Workflow.Claim(ctx, observation.Key())
	if err != nil {
		return ctrl.Result{}, err
	}
	if incident == nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	if err = r.Execute(ctx, &deployment, *incident); err != nil {
		// Transient API errors retain the execution key for bounded service retries.
		if !errors.Is(err, ErrUnsafePlan) {
			return ctrl.Result{}, err
		}
		if failErr := r.Workflow.Fail(ctx, incident.ID, err.Error()); failErr != nil {
			return ctrl.Result{}, fmt.Errorf("execution: %v; persist failure: %w", err, failErr)
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	if err = r.Workflow.MarkVerifying(ctx, incident.ID); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func (r *TargetReconciler) observe(d *appsv1.Deployment, container, image string) (core.Observation, error) {
	var current string
	for _, c := range d.Spec.Template.Spec.Containers {
		if c.Name == container {
			current = c.Image
		}
	}
	if current == "" {
		return core.Observation{}, fmt.Errorf("container %q does not exist", container)
	}
	desired := int32(1)
	if d.Spec.Replicas != nil {
		desired = *d.Spec.Replicas
	}
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	gitops := false
	for key := range d.Labels {
		if key == "argocd.argoproj.io/instance" || strings.HasPrefix(key, "kustomize.toolkit.fluxcd.io/") || strings.HasPrefix(key, "helm.toolkit.fluxcd.io/") {
			gitops = true
		}
	}
	if d.Annotations["argocd.argoproj.io/tracking-id"] != "" {
		gitops = true
	}
	return core.Observation{Cluster: r.ClusterID, Namespace: d.Namespace, Name: d.Name, UID: string(d.UID), ResourceVersion: d.ResourceVersion, Image: current, HealthyImage: image, Container: container, Generation: d.Generation, ObservedGeneration: d.Status.ObservedGeneration, Desired: desired, Available: d.Status.AvailableReplicas, Updated: d.Status.UpdatedReplicas, OptIn: d.Labels[OptInLabel] == "true", GitOps: gitops, At: now, ExecutionKey: d.Spec.Template.Annotations[ExecutionAnnotation]}, nil
}

type patchOperation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

// Execute uses Kubernetes JSON Patch compare-and-swap. UID, resourceVersion and
// original image are tested by the API server in the same atomic mutation. The
// approved UID, generation, image and replica count must remain unchanged; a
// status-only resourceVersion update permits a fresh read CAS. An
// execution annotation makes retry after a lost HTTP response idempotent.
func (r *TargetReconciler) Execute(ctx context.Context, d *appsv1.Deployment, incident core.Incident) error {
	if incident.State != "Executing" || incident.ExecutionKey == "" || incident.Plan.Action != "rollback" {
		return fmt.Errorf("%w: action must be executing and keyed", ErrUnsafePlan)
	}
	if !digestImage.MatchString(incident.Plan.Image) {
		return fmt.Errorf("%w: rollback image must be digest-pinned", ErrUnsafePlan)
	}
	target := incident.Plan.Target
	if d.Namespace != DemoNamespace || d.Namespace != target.Namespace || d.Name != target.Name || string(d.UID) != target.UID {
		return fmt.Errorf("%w: target identity changed", ErrUnsafePlan)
	}
	if d.Labels[OptInLabel] != "true" {
		return fmt.Errorf("%w: target no longer opted in", ErrUnsafePlan)
	}
	obs, err := r.observe(d, target.Container, incident.Plan.Image)
	if err != nil {
		return err
	}
	if obs.GitOps {
		return fmt.Errorf("%w: GitOps-owned workload requires configuration promotion, direct patch denied", ErrUnsafePlan)
	}
	index := -1
	for i, c := range d.Spec.Template.Spec.Containers {
		if c.Name == target.Container {
			index = i
		}
	}
	if index < 0 {
		return fmt.Errorf("%w: target container changed", ErrUnsafePlan)
	}
	if d.Spec.Template.Annotations[ExecutionAnnotation] == incident.ExecutionKey {
		if d.Spec.Template.Spec.Containers[index].Image != incident.Plan.Image {
			return fmt.Errorf("%w: execution marker exists but desired image drifted", ErrUnsafePlan)
		}
		return nil
	}
	if d.Spec.Template.Spec.Containers[index].Image != target.Image {
		return fmt.Errorf("%w: target image changed after planning", ErrUnsafePlan)
	}
	if d.Generation != target.Generation || obs.Desired != target.Desired {
		return fmt.Errorf("%w: target specification changed after planning", ErrUnsafePlan)
	}
	operations := []patchOperation{
		{"test", "/metadata/uid", target.UID},
		{"test", "/metadata/resourceVersion", d.ResourceVersion},
		{"test", fmt.Sprintf("/spec/template/spec/containers/%d/image", index), target.Image},
		{"replace", fmt.Sprintf("/spec/template/spec/containers/%d/image", index), incident.Plan.Image},
	}
	if d.Spec.Template.Annotations == nil {
		operations = append(operations, patchOperation{"add", "/spec/template/metadata/annotations", map[string]string{ExecutionAnnotation: incident.ExecutionKey}})
	} else {
		operations = append(operations, patchOperation{"add", "/spec/template/metadata/annotations/resilienceplane.io~1execution-key", incident.ExecutionKey})
	}
	payload, err := json.Marshal(operations)
	if err != nil {
		return err
	}
	return r.Client.Patch(ctx, d, client.RawPatch(types.JSONPatchType, payload))
}

// Target returns a typed-GVK unstructured resource for tests and callers.
func Target(namespace, name, deployment, container, image string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{"apiVersion": TargetGVK.GroupVersion().String(), "kind": TargetGVK.Kind, "metadata": map[string]any{"namespace": namespace, "name": name}, "spec": map[string]any{"deployment": deployment, "container": container, "healthyImage": image}}}
}
