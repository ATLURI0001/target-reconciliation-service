package core_test

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ATLURI0001/resilienceplane/internal/core"
)

func approvedFixture(t *testing.T) (*core.Service, *time.Time, core.Observation, core.Incident) {
	t.Helper()
	service, now, observation := fixture(t)
	incident := detect(t, service, now, observation)
	policy := core.DefaultPolicy()
	policy.DetectionSeconds = 5
	policy.DryRun = false
	if err := service.SetPolicy(ctx, operator, policy); err != nil {
		t.Fatal(err)
	}
	if err := service.Approve(ctx, operator, incident.ID); err != nil {
		t.Fatal(err)
	}
	return service, now, observation, incident
}
func TestRecoveredTargetDoesNotClaimApprovedIntervention(t *testing.T) {
	service, _, observation, incident := approvedFixture(t)
	observation.Available = observation.Desired
	observation.Updated = observation.Desired
	if err := service.Observe(ctx, observation); err != nil {
		t.Fatal(err)
	}
	claim, err := service.Claim(ctx, observation.Key())
	if err != nil {
		t.Fatal(err)
	}
	if claim != nil {
		t.Fatalf("healthy workload admitted for mutation: %+v", claim)
	}
	state, err := service.Store.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	terminal := state.Incidents[incident.ID]
	if terminal.State != "Failed" || terminal.ExecutionKey != "" || !strings.Contains(terminal.Error, "recovered") {
		t.Fatalf("intervention was not fenced: %+v", terminal)
	}
	if state.Observations[observation.Key()].Image != observation.Image {
		t.Fatal("fence changed target image")
	}
	for _, entry := range state.Audit {
		if entry.Resource == incident.ID && entry.Action == "incident.execute" {
			t.Fatal("recovered target received an execution record")
		}
	}
}
func TestLateHealthyEvidenceCannotBypassDeadline(t *testing.T) {
	service, now, observation, incident := approvedFixture(t)
	claim, err := service.Claim(ctx, observation.Key())
	if err != nil || claim == nil {
		t.Fatalf("claim: %v %v", claim, err)
	}
	if err = service.MarkVerifying(ctx, incident.ID); err != nil {
		t.Fatal(err)
	}
	*now = claim.Deadline
	observation.Image = incident.Plan.Image
	observation.ExecutionKey = claim.ExecutionKey
	observation.Generation++
	observation.ObservedGeneration = observation.Generation
	observation.Available = observation.Desired
	observation.Updated = observation.Desired
	if err = service.Observe(ctx, observation); err != nil {
		t.Fatal(err)
	}
	state, err := service.Store.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Incidents[incident.ID].State != "Failed" {
		t.Fatalf("late evidence accepted: %+v", state.Incidents[incident.ID])
	}
	for _, entry := range state.Audit {
		if entry.Resource == incident.ID && entry.Action == "incident.verify" {
			t.Fatal("late verification was recorded as successful")
		}
	}
}
func TestVerificationBindsApprovedContainerAndReplicaCount(t *testing.T) {
	for _, kind := range []string{"container", "replicas"} {
		t.Run(kind, func(t *testing.T) {
			service, _, observation, incident := approvedFixture(t)
			claim, err := service.Claim(ctx, observation.Key())
			if err != nil || claim == nil {
				t.Fatalf("claim: %v %v", claim, err)
			}
			if err = service.MarkVerifying(ctx, incident.ID); err != nil {
				t.Fatal(err)
			}
			observation.Image = incident.Plan.Image
			observation.ExecutionKey = claim.ExecutionKey
			observation.Generation++
			observation.ObservedGeneration = observation.Generation
			observation.Available = observation.Desired
			observation.Updated = observation.Desired
			if kind == "container" {
				observation.Container = "other"
			} else {
				observation.Desired = 1
				observation.Available = 1
				observation.Updated = 1
			}
			if err = service.Observe(ctx, observation); err != nil {
				t.Fatal(err)
			}
			state, err := service.Store.View(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if state.Incidents[incident.ID].State != "Verifying" {
				t.Fatalf("changed verification scope accepted: %+v", state.Incidents[incident.ID])
			}
		})
	}
}
func TestInvalidPolicyViewerDeniedAndAuditedBeforeValidation(t *testing.T) {
	service, _, _ := fixture(t)
	before, err := service.Store.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	viewer := core.Identity{Subject: "invalid-policy-reader", Role: "viewer"}
	if err = service.SetPolicy(ctx, viewer, core.Policy{}); !errors.Is(err, core.ErrDenied) {
		t.Fatalf("expected authorization denial, got %v", err)
	}
	after, err := service.Store.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Audit) != len(before.Audit)+1 {
		t.Fatal("invalid unauthorized request missing audit")
	}
	entry := after.Audit[len(after.Audit)-1]
	if entry.Actor != viewer.Subject || entry.Action != "policy.update" || entry.Result != "denied" {
		t.Fatalf("unexpected denial record: %+v", entry)
	}
	if after.Policy.DetectionSeconds != before.Policy.DetectionSeconds || after.Policy.DryRun != before.Policy.DryRun {
		t.Fatal("unauthorized request changed policy")
	}
}
func TestDetectionWindowBoundsPreventDurationOverflow(t *testing.T) {
	for _, seconds := range []int{0, 86401, int(^uint(0) >> 1)} {
		policy := core.DefaultPolicy()
		policy.DetectionSeconds = seconds
		if err := policy.Validate(); !errors.Is(err, core.ErrInvalid) {
			t.Fatalf("invalid detection window %d accepted: %v", seconds, err)
		}
	}
	policy := core.DefaultPolicy()
	policy.DetectionSeconds = 86400
	if err := policy.Validate(); err != nil {
		t.Fatalf("maximum detection window rejected: %v", err)
	}
}
func TestConcurrentClaimsReplayOnePerTargetExecution(t *testing.T) {
	service, _, observation, incident := approvedFixture(t)
	// Seed another approved proposal for the same target to exercise admission's
	// conflict guard. It represents overlapping incident creation after a new
	// unhealthy interval; only one operation may become active on this target.
	if err := service.Store.Update(ctx, func(state *core.State) error {
		policy := state.Policy
		policy.MaxConcurrent = 10
		state.Policy = policy
		second := state.Incidents[incident.ID]
		second.ID = "competing-incident"
		state.Incidents[second.ID] = second
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	keys := make(chan string, 24)
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			claim, err := service.Claim(ctx, observation.Key())
			if err != nil {
				t.Error(err)
				return
			}
			if claim != nil {
				keys <- claim.ExecutionKey
			}
		}()
	}
	wg.Wait()
	close(keys)
	key := ""
	for got := range keys {
		if key == "" {
			key = got
		}
		if got != key {
			t.Fatalf("competing execution keys admitted: %s and %s", key, got)
		}
	}
	if key == "" {
		t.Fatal("no operation admitted")
	}
	state, err := service.Store.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	active := 0
	executions := 0
	for _, item := range state.Incidents {
		if item.Target == observation.Key() && item.State == "Executing" {
			active++
		}
	}
	for _, entry := range state.Audit {
		if entry.Action == "incident.execute" {
			executions++
		}
	}
	if active != 1 || executions != 1 {
		t.Fatalf("per-target admission failed: active=%d execution records=%d", active, executions)
	}
	if err = service.MarkVerifying(ctx, key); err != nil {
		t.Fatal(err)
	}
	if claim, err := service.Claim(ctx, observation.Key()); err != nil || claim != nil {
		t.Fatalf("competing proposal admitted during verification: %+v %v", claim, err)
	}
}
