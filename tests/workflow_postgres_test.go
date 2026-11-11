package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ATLURI0001/resilienceplane/internal/core"
	"github.com/ATLURI0001/resilienceplane/internal/store"
)

// TestPostgresWorkflow exercises the actual durable service and SQL repository.
// Observations are synthetic test input; this test does not exercise Kubernetes.
func TestPostgresWorkflow(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required; actual PostgreSQL workflow not executed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repository, err := store.NewPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	service := core.NewService(repository)
	now := time.Now().UTC()
	service.Now = func() time.Time { return now }
	operator := core.Identity{Subject: "workflow-test-operator", Role: "operator"}
	viewer := core.Identity{Subject: "workflow-test-viewer", Role: "viewer"}
	clusterID := fmt.Sprintf("workflow-%d", time.Now().UnixNano())
	if err = service.Register(ctx, operator, core.Cluster{ID: clusterID, Environment: "test"}); err != nil {
		t.Fatal(err)
	}
	policy := core.DefaultPolicy()
	policy.DetectionSeconds = 1
	policy.DryRun = false
	if err = service.SetPolicy(ctx, operator, policy); err != nil {
		t.Fatal(err)
	}
	recoveryImage := "example.invalid/catalog@sha256:" + strings.Repeat("a", 64)
	observation := core.Observation{Cluster: clusterID, Namespace: "rp-demo", Name: "catalog", UID: "workflow-test-uid", ResourceVersion: "1", Image: "example.invalid/catalog:failed-release", HealthyImage: recoveryImage, Container: "catalog", Generation: 2, ObservedGeneration: 2, Desired: 1, Available: 0, Updated: 0, OptIn: true}
	if err = service.Observe(ctx, observation); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if err = service.Observe(ctx, observation); err != nil {
		t.Fatal(err)
	}
	state, err := repository.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var incident core.Incident
	for _, item := range state.Incidents {
		if item.Target == observation.Key() {
			incident = item
			break
		}
	}
	if incident.ID == "" || incident.State != "AwaitingApproval" {
		t.Fatalf("detection did not propose approval: %+v", incident)
	}
	if err = service.Approve(ctx, viewer, incident.ID); !errors.Is(err, core.ErrDenied) {
		t.Fatalf("viewer approval expected denied, got %v", err)
	}
	state, err = repository.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Incidents[incident.ID].State != "AwaitingApproval" {
		t.Fatal("unauthorized approval changed incident state")
	}
	if err = service.Approve(ctx, operator, incident.ID); err != nil {
		t.Fatal(err)
	}
	claimed, err := service.Claim(ctx, observation.Key())
	if err != nil {
		t.Fatal(err)
	}
	if claimed == nil || claimed.ExecutionKey == "" || claimed.State != "Executing" {
		t.Fatalf("execution not claimed: %+v", claimed)
	}
	// A new connection pool and service emulate persisted workflow restart. No
	// process crash or Kubernetes API execution is implied by this boundary.
	reconnected, err := store.NewPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer reconnected.Close()
	resumed := core.NewService(reconnected)
	resumed.Now = func() time.Time { return now }
	replay, err := resumed.Claim(ctx, observation.Key())
	if err != nil {
		t.Fatal(err)
	}
	if replay == nil || replay.ExecutionKey != claimed.ExecutionKey {
		t.Fatalf("durable operation key changed across service reconstruction: %+v", replay)
	}
	if err = resumed.MarkVerifying(ctx, incident.ID); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	observation.ResourceVersion = "2"
	observation.Image = recoveryImage
	observation.ExecutionKey = claimed.ExecutionKey
	observation.Generation = 3
	observation.ObservedGeneration = 3
	observation.Available = 1
	observation.Updated = 1
	if err = resumed.Observe(ctx, observation); err != nil {
		t.Fatal(err)
	}
	final, err := repository.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if final.Incidents[incident.ID].State != "Resolved" {
		t.Fatalf("post-action verification: %+v", final.Incidents[incident.ID])
	}
	expected := map[string]bool{"incident.detect": false, "plan.evaluate": false, "incident.approve": false, "incident.execute": false, "incident.transition": false, "incident.verify": false}
	denialFound := false
	for _, entry := range final.Audit {
		if entry.Resource != incident.ID {
			continue
		}
		if _, ok := expected[entry.Action]; ok {
			expected[entry.Action] = true
		}
		if entry.Actor == viewer.Subject && entry.Action == "incident.approve" && entry.Result == "denied" {
			denialFound = true
		}
	}
	for action, found := range expected {
		if !found {
			t.Errorf("durable audit missing %s", action)
		}
	}
	if !denialFound {
		t.Fatal("authorization denial was not persistently audited")
	}
	t.Logf("actual PostgreSQL workflow resolved incident; stable execution key across reconstructed service; denied viewer approval audited")
}
