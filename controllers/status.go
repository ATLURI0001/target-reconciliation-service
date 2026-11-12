package controllers

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/ATLURI0001/target-reconciliation-service/internal/core"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// targetStatus is an observation result, not an authorization decision.
type targetStatus struct {
	Ready      string
	Reconciled string
	Reason     string
	Message    string
	Evidence   map[string]any
}

func unknownStatus(reason, message string) targetStatus {
	return targetStatus{Ready: "Unknown", Reconciled: "False", Reason: reason, Message: message}
}
func (s targetStatus) failed(reason, message string) targetStatus {
	s.Ready = "Unknown"
	s.Reconciled = "False"
	s.Reason = reason
	s.Message = message
	return s
}
func observedStatus(o core.Observation) targetStatus {
	s := targetStatus{Ready: "False", Reconciled: "True", Reason: "RolloutIncomplete", Message: "Live Deployment rollout is incomplete: total/updated replicas must equal desired and all desired replicas must be available"}
	if o.Healthy() {
		s.Ready = "True"
		s.Reason = "DeploymentReady"
		s.Message = "Live Deployment has observed its generation and all desired updated replicas are available"
	}
	s.Evidence = map[string]any{"deploymentUID": o.UID, "generation": o.Generation, "observedGeneration": o.ObservedGeneration, "desired": int64(o.Desired), "replicas": int64(o.Replicas), "available": int64(o.Available), "updated": int64(o.Updated), "image": o.Image}
	return s
}

func (r *TargetReconciler) finish(ctx context.Context, target *unstructured.Unstructured, status targetStatus, original error) (ctrl.Result, error) {
	err := r.publishStatus(ctx, target, status)
	if err != nil {
		original = errors.Join(original, fmt.Errorf("publish ReliabilityTarget status: %w", err))
	}
	if original != nil {
		return ctrl.Result{}, original
	}
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func (r *TargetReconciler) publishStatus(ctx context.Context, observed *unstructured.Unstructured, result targetStatus) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		live := &unstructured.Unstructured{}
		live.SetGroupVersionKind(TargetGVK)
		if err := r.reader().Get(ctx, client.ObjectKeyFromObject(observed), live); err != nil {
			return err
		}
		// Never attach old evidence to a replacement resource or newer desired spec.
		if live.GetUID() != observed.GetUID() || live.GetGeneration() != observed.GetGeneration() {
			return fmt.Errorf("target identity or generation changed during reconciliation; fresh observation required")
		}
		before := live.DeepCopy()
		var conditions []metav1.Condition
		raw, exists, err := unstructured.NestedSlice(live.Object, "status", "conditions")
		if err != nil {
			return err
		}
		if exists {
			for _, item := range raw {
				m, ok := item.(map[string]any)
				if !ok {
					return fmt.Errorf("invalid condition object")
				}
				var c metav1.Condition
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(m, &c); err != nil {
					return err
				}
				conditions = append(conditions, c)
			}
		}
		now := metav1.Now()
		if r.Now != nil {
			now = metav1.NewTime(r.Now())
		}
		// SetStatusCondition only changes lastTransitionTime when condition Status
		// changes. Reason/message and observedGeneration updates retain that time.
		for _, c := range []metav1.Condition{
			{Type: "Ready", Status: metav1.ConditionStatus(result.Ready), ObservedGeneration: observed.GetGeneration(), Reason: result.Reason, Message: result.Message, LastTransitionTime: now},
			{Type: "Reconciled", Status: metav1.ConditionStatus(result.Reconciled), ObservedGeneration: observed.GetGeneration(), Reason: result.Reason, Message: result.Message, LastTransitionTime: now},
		} {
			apimeta.SetStatusCondition(&conditions, c)
		}
		serialized := make([]any, 0, len(conditions))
		for i := range conditions {
			m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&conditions[i])
			if err != nil {
				return err
			}
			serialized = append(serialized, m)
		}
		if err := unstructured.SetNestedSlice(live.Object, serialized, "status", "conditions"); err != nil {
			return err
		}
		if err := unstructured.SetNestedField(live.Object, observed.GetGeneration(), "status", "observedGeneration"); err != nil {
			return err
		}
		if result.Evidence != nil {
			if err := unstructured.SetNestedMap(live.Object, result.Evidence, "status", "lastObservation"); err != nil {
				return err
			}
		}
		old, _, _ := unstructured.NestedMap(before.Object, "status")
		updated, _, _ := unstructured.NestedMap(live.Object, "status")
		if reflect.DeepEqual(old, updated) {
			return nil
		}
		return r.Client.Status().Patch(ctx, live, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{}))
	})
}
