package core_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ATLURI0001/target-reconciliation-service/internal/core"
)

func TestParseTargetKey(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		cluster   string
		namespace string
		target    string
		wantErr   bool
	}{
		{
			name:      "valid key",
			input:     "cluster-alpha/trs-demo/worker-deploy",
			cluster:   "cluster-alpha",
			namespace: "trs-demo",
			target:    "worker-deploy",
			wantErr:   false,
		},
		{
			name:      "valid key with whitespace",
			input:     "  cluster-beta/trs-demo/api-deploy  ",
			cluster:   "cluster-beta",
			namespace: "trs-demo",
			target:    "api-deploy",
			wantErr:   false,
		},
		{
			name:    "missing segment",
			input:   "cluster-alpha/trs-demo",
			wantErr: true,
		},
		{
			name:    "empty segment",
			input:   "cluster-alpha//worker-deploy",
			wantErr: true,
		},
		{
			name:    "whitespace segment",
			input:   "cluster-alpha/   /worker-deploy",
			wantErr: true,
		},
		{
			name:    "too many segments",
			input:   "cluster-alpha/trs-demo/worker/extra",
			wantErr: true,
		},
		{
			name:    "blank input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, ns, n, err := core.ParseTargetKey(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseTargetKey(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
			if !tc.wantErr {
				if c != tc.cluster || ns != tc.namespace || n != tc.target {
					t.Fatalf("ParseTargetKey(%q) = (%q, %q, %q), want (%q, %q, %q)",
						tc.input, c, ns, n, tc.cluster, tc.namespace, tc.target)
				}
			} else {
				if !errors.Is(err, core.ErrInvalid) {
					t.Fatalf("expected ErrInvalid, got %v", err)
				}
			}
		})
	}
}

func TestObservationTargetMatches(t *testing.T) {
	obs := core.Observation{
		Cluster:   "east-1",
		Namespace: "trs-demo",
		Name:      "web-svc",
	}

	if !obs.TargetMatches("east-1", "trs-demo", "web-svc") {
		t.Fatal("expected TargetMatches to return true for identical components")
	}
	if obs.TargetMatches("west-1", "trs-demo", "web-svc") {
		t.Fatal("expected TargetMatches to return false for mismatched cluster")
	}
	if obs.TargetMatches("east-1", "other", "web-svc") {
		t.Fatal("expected TargetMatches to return false for mismatched namespace")
	}
	if obs.TargetMatches("east-1", "trs-demo", "other") {
		t.Fatal("expected TargetMatches to return false for mismatched name")
	}
}

func TestIncidentIsTerminalAndTimeRemaining(t *testing.T) {
	terminalStates := []string{
		"verified", "aborted", "timed_out", "rejected",
		"Resolved", "Cancelled", "Failed", "Rejected",
		" resolved ", "FAILED",
	}
	for _, st := range terminalStates {
		inc := core.Incident{State: st}
		if !inc.IsTerminal() {
			t.Fatalf("state %q should be terminal", st)
		}
	}

	activeStates := []string{"detected", "pending_approval", "approved", "reconciling", "Executing", "Verifying", "AwaitingApproval"}
	for _, st := range activeStates {
		inc := core.Incident{State: st}
		if inc.IsTerminal() {
			t.Fatalf("state %q should not be terminal", st)
		}
	}

	now := time.Now()
	// Zero deadline
	incNoDeadline := core.Incident{}
	if rem := incNoDeadline.TimeRemaining(now); rem != 0 {
		t.Fatalf("expected 0 for zero deadline, got %v", rem)
	}

	// Future deadline
	incFuture := core.Incident{Deadline: now.Add(45 * time.Second)}
	if rem := incFuture.TimeRemaining(now); rem != 45*time.Second {
		t.Fatalf("expected 45s, got %v", rem)
	}

	// Past deadline
	incPast := core.Incident{Deadline: now.Add(-10 * time.Second)}
	if rem := incPast.TimeRemaining(now); rem != 0 {
		t.Fatalf("expected 0 for expired deadline, got %v", rem)
	}
}

func TestPolicySummary(t *testing.T) {
	p := core.DefaultPolicy()
	summary := p.Summary()

	if !strings.Contains(summary, "namespaces=[trs-demo]") {
		t.Fatalf("expected namespaces in summary, got %q", summary)
	}
	if !strings.Contains(summary, "maxReplicas=3") {
		t.Fatalf("expected maxReplicas in summary, got %q", summary)
	}
	if !strings.Contains(summary, "dryRun=true") {
		t.Fatalf("expected dryRun in summary, got %q", summary)
	}
}

func TestPolicyValidateGuards(t *testing.T) {
	p := core.DefaultPolicy()
	p.Namespaces = []string{"   "}
	if err := p.Validate(); err == nil {
		t.Fatal("expected Validate to reject whitespace-only namespace")
	}

	p.Namespaces = []string{" kube-system "}
	if err := p.Validate(); err == nil {
		t.Fatal("expected Validate to reject padded protected namespace")
	}

	p.Namespaces = []string{"trs-demo", "trs-demo"}
	if err := p.Validate(); err == nil {
		t.Fatal("expected Validate to reject duplicate namespaces")
	}
}

func TestPolicyAllowsNamespace(t *testing.T) {
	p := core.Policy{
		Namespaces: []string{"trs-demo", "staging"},
	}

	// Allowed namespaces with and without whitespace
	if !p.AllowsNamespace("trs-demo") {
		t.Fatal("expected trs-demo to be allowed")
	}
	if !p.AllowsNamespace("  trs-demo  ") {
		t.Fatal("expected padded trs-demo to be allowed")
	}
	if !p.AllowsNamespace("staging") {
		t.Fatal("expected staging to be allowed")
	}

	// Disallowed namespaces
	if p.AllowsNamespace("production") {
		t.Fatal("expected unconfigured production to be disallowed")
	}
	if p.AllowsNamespace("") {
		t.Fatal("expected empty namespace to be disallowed")
	}
	if p.AllowsNamespace("   ") {
		t.Fatal("expected whitespace namespace to be disallowed")
	}

	// Protected namespaces must always be rejected even if configured
	pProt := core.Policy{
		Namespaces: []string{"kube-system", "default", "target-reconciliation-service"},
	}
	if pProt.AllowsNamespace("kube-system") {
		t.Fatal("expected kube-system to be rejected")
	}
	if pProt.AllowsNamespace("default") {
		t.Fatal("expected default to be rejected")
	}
	if pProt.AllowsNamespace("target-reconciliation-service") {
		t.Fatal("expected target-reconciliation-service to be rejected")
	}
}

func TestObservationValidate(t *testing.T) {
	valid := core.Observation{
		Cluster:   "west-1",
		Namespace: "trs-demo",
		Name:      "web",
		Desired:   2,
		Replicas:  2,
		Available: 2,
		Updated:   2,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid observation to pass, got %v", err)
	}

	invalidCases := []struct {
		name string
		obs  core.Observation
	}{
		{"empty cluster", core.Observation{Cluster: "", Namespace: "demo", Name: "app"}},
		{"whitespace cluster", core.Observation{Cluster: "   ", Namespace: "demo", Name: "app"}},
		{"empty namespace", core.Observation{Cluster: "west-1", Namespace: "", Name: "app"}},
		{"empty name", core.Observation{Cluster: "west-1", Namespace: "demo", Name: ""}},
		{"negative desired", core.Observation{Cluster: "west-1", Namespace: "demo", Name: "app", Desired: -1}},
		{"negative replicas", core.Observation{Cluster: "west-1", Namespace: "demo", Name: "app", Replicas: -2}},
		{"negative available", core.Observation{Cluster: "west-1", Namespace: "demo", Name: "app", Available: -1}},
		{"negative updated", core.Observation{Cluster: "west-1", Namespace: "demo", Name: "app", Updated: -1}},
	}
	for _, tc := range invalidCases {
		if err := tc.obs.Validate(); err == nil {
			t.Errorf("expected error for case %q, got nil", tc.name)
		}
	}
}

func TestPolicyAllowsReplicas(t *testing.T) {
	p := core.Policy{MaxReplicas: 5}
	if !p.AllowsReplicas(0) {
		t.Error("expected 0 replicas to be allowed")
	}
	if !p.AllowsReplicas(5) {
		t.Error("expected 5 replicas to be allowed")
	}
	if p.AllowsReplicas(6) {
		t.Error("expected 6 replicas to exceed max")
	}
	if p.AllowsReplicas(-1) {
		t.Error("expected negative replicas to be disallowed")
	}
}

func TestStateActiveIncidents(t *testing.T) {
	st := core.NewState()
	if st.ActiveIncidents() != 0 {
		t.Fatalf("expected 0 active incidents initially, got %d", st.ActiveIncidents())
	}

	st.Incidents["inc-1"] = core.Incident{ID: "inc-1", State: "pending_approval"}
	st.Incidents["inc-2"] = core.Incident{ID: "inc-2", State: "resolved"}
	st.Incidents["inc-3"] = core.Incident{ID: "inc-3", State: "reconciling"}
	st.Incidents["inc-4"] = core.Incident{ID: "inc-4", State: "cancelled"}

	if active := st.ActiveIncidents(); active != 2 {
		t.Fatalf("expected 2 active incidents, got %d", active)
	}
}
