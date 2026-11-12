package core_test

import (
	"github.com/ATLURI0001/target-reconciliation-service/internal/core"
	"testing"
)

// A Deployment's updatedReplicas can include an unready new Pod while
// availableReplicas counts an old Pod. Require complete rollout convergence.
func TestHealthyRequiresNoOldReplicaMaskingFailedRelease(t *testing.T) {
	o := core.Observation{Desired: 1, Replicas: 2, Updated: 1, Available: 1, Generation: 2, ObservedGeneration: 2}
	if o.Healthy() {
		t.Fatal("old available replica masked unready new release")
	}
	o.Replicas = 1
	if !o.Healthy() {
		t.Fatal("fully converged deployment not healthy")
	}
	o.Updated = 2
	if o.Healthy() {
		t.Fatal("surplus updated replicas accepted")
	}
	o.Updated = 1
	o.ObservedGeneration = 1
	if o.Healthy() {
		t.Fatal("old generation accepted")
	}
}
