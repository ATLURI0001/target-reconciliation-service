package core_test

import (
	"context"
	"errors"
	"github.com/ATLURI0001/target-reconciliation-service/internal/core"
	"github.com/ATLURI0001/target-reconciliation-service/internal/store"
	"strings"
	"testing"
	"time"
)

var ctx = context.Background()
var operator = core.Identity{Subject: "test-operator", Role: "operator"}

func fixture(t *testing.T) (*core.Service, *time.Time, core.Observation) {
	t.Helper()
	s := core.NewService(store.NewMemory())
	now := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }
	if e := s.Register(ctx, operator, core.Cluster{ID: "local", Environment: "test"}); e != nil {
		t.Fatal(e)
	}
	p := core.DefaultPolicy()
	p.DetectionSeconds = 5
	if e := s.SetPolicy(ctx, operator, p); e != nil {
		t.Fatal(e)
	}
	o := core.Observation{Cluster: "local", Namespace: "trs-demo", Name: "catalog", UID: "uid", ResourceVersion: "1", Image: "bad:v2", HealthyImage: "example/catalog@sha256:" + strings.Repeat("a", 64), Container: "catalog", Generation: 2, ObservedGeneration: 2, Desired: 2, Replicas: 2, OptIn: true}
	return s, &now, o
}
func detect(t *testing.T, s *core.Service, now *time.Time, o core.Observation) core.Incident {
	t.Helper()
	if e := s.Observe(ctx, o); e != nil {
		t.Fatal(e)
	}
	*now = now.Add(6 * time.Second)
	if e := s.Observe(ctx, o); e != nil {
		t.Fatal(e)
	}
	st, e := s.Store.View(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for _, i := range st.Incidents {
		return i
	}
	t.Fatal("no incident")
	return core.Incident{}
}
func TestApprovedRecoveryLifecycle(t *testing.T) {
	s, now, o := fixture(t)
	i := detect(t, s, now, o)
	if i.State != "AwaitingApproval" {
		t.Fatal(i.State)
	}
	if e := s.Approve(ctx, core.Identity{Subject: "reader", Role: "viewer"}, i.ID); !errors.Is(e, core.ErrDenied) {
		t.Fatal(e)
	}
	if e := s.Approve(ctx, operator, i.ID); e != nil {
		t.Fatal(e)
	}
	if v, e := s.Claim(ctx, o.Key()); e != nil || v != nil {
		t.Fatalf("dry run claimed: %v %v", v, e)
	}
	p := core.DefaultPolicy()
	p.DryRun = false
	p.DetectionSeconds = 5
	if e := s.SetPolicy(ctx, operator, p); e != nil {
		t.Fatal(e)
	}
	if e := s.Approve(ctx, operator, i.ID); e != nil {
		t.Fatal(e)
	}
	claim, e := s.Claim(ctx, o.Key())
	if e != nil || claim == nil {
		t.Fatalf("claim %v %v", claim, e)
	}
	restarted := core.NewService(s.Store)
	restarted.Now = s.Now
	again, e := restarted.Claim(ctx, o.Key())
	if e != nil || again.ExecutionKey != claim.ExecutionKey {
		t.Fatal("replay changed key", e)
	}
	if e = s.MarkVerifying(ctx, i.ID); e != nil {
		t.Fatal(e)
	}
	o.Image = i.Plan.Image
	o.Generation = 3
	o.ObservedGeneration = 3
	o.Available = 2
	o.Updated = 2
	if e = s.Observe(ctx, o); e != nil {
		t.Fatal(e)
	}
	st, _ := s.Store.View(ctx)
	if st.Incidents[i.ID].State != "Verifying" {
		t.Fatal("unattributed success accepted")
	}
	o.ExecutionKey = claim.ExecutionKey
	if e = s.Observe(ctx, o); e != nil {
		t.Fatal(e)
	}
	st, _ = s.Store.View(ctx)
	if st.Incidents[i.ID].State != "Resolved" {
		t.Fatal(st.Incidents[i.ID])
	}
	if e = s.Approve(ctx, operator, i.ID); !errors.Is(e, core.ErrConflict) {
		t.Fatal("terminal transition", e)
	}
	denied := false
	for _, a := range st.Audit {
		if a.Actor == "reader" && a.Result == "denied" {
			denied = true
		}
	}
	if !denied {
		t.Fatal("denial not audited")
	}
}
func TestPolicyFences(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*core.Observation)
	}{
		{"protected", func(o *core.Observation) { o.Namespace = "kube-system" }}, {"not-opted-in", func(o *core.Observation) { o.OptIn = false }}, {"gitops", func(o *core.Observation) { o.GitOps = true }}, {"blast-radius", func(o *core.Observation) { o.Desired = 4 }}, {"mutable-image", func(o *core.Observation) { o.HealthyImage = "catalog:latest" }}, {"ordinary-healing", func(o *core.Observation) { o.Image = o.HealthyImage }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, now, o := fixture(t)
			tc.change(&o)
			i := detect(t, s, now, o)
			if i.State != "Rejected" {
				t.Fatal(i)
			}
			if c, e := s.Claim(ctx, o.Key()); e != nil || c != nil {
				t.Fatal("unsafe claim", c, e)
			}
		})
	}
}
func TestFreshnessAndEmergencyStop(t *testing.T) {
	s, now, o := fixture(t)
	i := detect(t, s, now, o)
	*now = now.Add(31 * time.Second)
	if e := s.Approve(ctx, operator, i.ID); !errors.Is(e, core.ErrDenied) {
		t.Fatal("stale approval", e)
	}
	if e := s.Observe(ctx, o); e != nil {
		t.Fatal(e)
	}
	p := core.DefaultPolicy()
	p.DryRun = false
	p.EmergencyStop = true
	if e := s.SetPolicy(ctx, operator, p); e != nil {
		t.Fatal(e)
	}
	if e := s.Approve(ctx, operator, i.ID); !errors.Is(e, core.ErrDenied) {
		t.Fatal("stop ignored", e)
	}
}
func TestTimeoutAndCancellation(t *testing.T) {
	s, now, o := fixture(t)
	i := detect(t, s, now, o)
	p := core.DefaultPolicy()
	p.DryRun = false
	if e := s.SetPolicy(ctx, operator, p); e != nil {
		t.Fatal(e)
	}
	if e := s.Approve(ctx, operator, i.ID); e != nil {
		t.Fatal(e)
	}
	c, e := s.Claim(ctx, o.Key())
	if e != nil || c == nil {
		t.Fatal(c, e)
	}
	*now = now.Add(121 * time.Second)
	if c, e = s.Claim(ctx, o.Key()); e != nil || c != nil {
		t.Fatal(c, e)
	}
	st, _ := s.Store.View(ctx)
	if st.Incidents[i.ID].State != "Failed" {
		t.Fatal("deadline ignored")
	}
	s, now, o = fixture(t)
	i = detect(t, s, now, o)
	if e = s.Cancel(ctx, operator, i.ID); e != nil {
		t.Fatal(e)
	}
	if c, e = s.Claim(ctx, o.Key()); e != nil || c != nil {
		t.Fatal("cancel claimed")
	}
}
func TestNoIncidentAcrossTelemetryGap(t *testing.T) {
	s, now, o := fixture(t)
	_ = s.Observe(ctx, o)
	*now = now.Add(time.Minute)
	_ = s.Observe(ctx, o)
	st, _ := s.Store.View(ctx)
	if len(st.Incidents) != 0 {
		t.Fatal("stale duration counted as evidence")
	}
}
func TestPolicyValidation(t *testing.T) {
	p := core.DefaultPolicy()
	if p.Validate() != nil {
		t.Fatal("default invalid")
	}
	p.Namespaces = []string{"kube-system"}
	if p.Validate() == nil {
		t.Fatal("protected namespace accepted")
	}
}
