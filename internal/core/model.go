package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid request")
var ErrDenied = errors.New("operation denied")
var ErrConflict = errors.New("state conflict")
var ErrNotFound = errors.New("not found")

// Identity comes from server-side authentication, never a request body.
type Identity struct {
	Subject string `json:"subject"`
	Role    string `json:"role"`
}
type Cluster struct {
	ID          string    `json:"id"`
	Environment string    `json:"environment"`
	LastSeen    time.Time `json:"lastSeen"`
	Connected   bool      `json:"connected"`
}
type Observation struct {
	Cluster            string    `json:"cluster"`
	Namespace          string    `json:"namespace"`
	Name               string    `json:"name"`
	UID                string    `json:"uid"`
	ResourceVersion    string    `json:"resourceVersion"`
	Image              string    `json:"image"`
	HealthyImage       string    `json:"healthyImage"`
	Container          string    `json:"container"`
	Generation         int64     `json:"generation"`
	ObservedGeneration int64     `json:"observedGeneration"`
	Desired            int32     `json:"desired"`
	Replicas           int32     `json:"replicas"`
	Available          int32     `json:"available"`
	Updated            int32     `json:"updated"`
	OptIn              bool      `json:"optIn"`
	GitOps             bool      `json:"gitops"`
	At                 time.Time `json:"at"`
	ExecutionKey       string    `json:"executionKey,omitempty"`
}

func (o Observation) Key() string { return o.Cluster + "/" + o.Namespace + "/" + o.Name }
func (o Observation) Healthy() bool {
	return o.Desired > 0 && o.Replicas == o.Desired && o.Available >= o.Desired && o.Updated == o.Desired && o.ObservedGeneration >= o.Generation
}

// ParseTargetKey splits a "cluster/namespace/name" formatted target key into its components.
func ParseTargetKey(key string) (cluster, namespace, name string, err error) {
	parts := strings.Split(strings.TrimSpace(key), "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("%w: target key must follow cluster/namespace/name format", ErrInvalid)
	}
	return parts[0], parts[1], parts[2], nil
}

// TargetMatches checks whether the observation matches the specified cluster, namespace, and name.
func (o Observation) TargetMatches(cluster, namespace, name string) bool {
	return o.Cluster == cluster && o.Namespace == namespace && o.Name == name
}

type Plan struct {
	Action           string      `json:"action"`
	Reason           string      `json:"reason"`
	Target           Observation `json:"target"`
	Image            string      `json:"image"`
	RequiresApproval bool        `json:"requiresApproval"`
	Verification     string      `json:"verification"`
	Abort            string      `json:"abort"`
}
type Incident struct {
	ID           string    `json:"id"`
	Target       string    `json:"target"`
	State        string    `json:"state"`
	Plan         Plan      `json:"plan"`
	Created      time.Time `json:"created"`
	Updated      time.Time `json:"updated"`
	ApprovedBy   string    `json:"approvedBy,omitempty"`
	ExecutionKey string    `json:"executionKey,omitempty"`
	Deadline     time.Time `json:"deadline,omitempty"`
	Error        string    `json:"error,omitempty"`
}

// IsTerminal returns true if the incident has reached a terminal state.
func (i Incident) IsTerminal() bool {
	switch i.State {
	case "verified", "aborted", "timed_out", "rejected":
		return true
	default:
		return false
	}
}

// TimeRemaining returns the duration remaining until the incident deadline, or 0 if expired or not set.
func (i Incident) TimeRemaining(now time.Time) time.Duration {
	if i.Deadline.IsZero() || !now.Before(i.Deadline) {
		return 0
	}
	return i.Deadline.Sub(now)
}
type Audit struct {
	Sequence int       `json:"sequence"`
	At       time.Time `json:"at"`
	Actor    string    `json:"actor"`
	Action   string    `json:"action"`
	Resource string    `json:"resource"`
	Result   string    `json:"result"`
}
type Policy struct {
	Namespaces       []string `json:"namespaces"`
	MaxReplicas      int32    `json:"maxReplicas"`
	MaxConcurrent    int      `json:"maxConcurrent"`
	FreshnessSeconds int      `json:"freshnessSeconds"`
	DetectionSeconds int      `json:"detectionSeconds"`
	DryRun           bool     `json:"dryRun"`
	EmergencyStop    bool     `json:"emergencyStop"`
}

func (p Policy) Validate() error {
	if len(p.Namespaces) == 0 || p.MaxReplicas < 1 || p.MaxConcurrent < 1 || p.MaxConcurrent > 10 || p.FreshnessSeconds < 5 || p.FreshnessSeconds > 300 || p.DetectionSeconds < 1 || p.DetectionSeconds > 86400 {
		return fmt.Errorf("%w: invalid safety policy", ErrInvalid)
	}
	for _, n := range p.Namespaces {
		if n == "" || n == "default" || n == "kube-system" || n == "target-reconciliation-service" {
			return fmt.Errorf("%w: protected namespace", ErrInvalid)
		}
	}
	return nil
}
func DefaultPolicy() Policy {
	return Policy{Namespaces: []string{"trs-demo"}, MaxReplicas: 3, MaxConcurrent: 1, FreshnessSeconds: 30, DetectionSeconds: 30, DryRun: true}
}

// Summary returns a concise diagnostic string representation of safety policy limits.
func (p Policy) Summary() string {
	return fmt.Sprintf("namespaces=%v, maxReplicas=%d, maxConcurrent=%d, freshness=%ds, detection=%ds, dryRun=%t, emergencyStop=%t",
		p.Namespaces, p.MaxReplicas, p.MaxConcurrent, p.FreshnessSeconds, p.DetectionSeconds, p.DryRun, p.EmergencyStop)
}

type State struct {
	Clusters       map[string]Cluster     `json:"clusters"`
	Observations   map[string]Observation `json:"observations"`
	UnhealthySince map[string]time.Time   `json:"unhealthySince"`
	Incidents      map[string]Incident    `json:"incidents"`
	Audit          []Audit                `json:"audit"`
	Policy         Policy                 `json:"policy"`
}

func NewState() State {
	return State{Clusters: map[string]Cluster{}, Observations: map[string]Observation{}, UnhealthySince: map[string]time.Time{}, Incidents: map[string]Incident{}, Audit: []Audit{}, Policy: DefaultPolicy()}
}

// Update must be atomic; callback errors roll back all changes. View returns an isolated snapshot.
type Store interface {
	Update(context.Context, func(*State) error) error
	View(context.Context) (State, error)
}
