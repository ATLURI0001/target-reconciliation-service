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
	terminalStates := []string{"verified", "aborted", "timed_out", "rejected"}
	for _, st := range terminalStates {
		inc := core.Incident{State: st}
		if !inc.IsTerminal() {
			t.Fatalf("state %q should be terminal", st)
		}
	}

	activeStates := []string{"detected", "pending_approval", "approved", "reconciling"}
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
