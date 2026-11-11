package core

import (
	"context"
	"errors"
	"fmt"
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
	Available          int32     `json:"available"`
	Updated            int32     `json:"updated"`
	OptIn              bool      `json:"optIn"`
	GitOps             bool      `json:"gitops"`
	At                 time.Time `json:"at"`
	ExecutionKey       string    `json:"executionKey,omitempty"`
}

func (o Observation) Key() string { return o.Cluster + "/" + o.Namespace + "/" + o.Name }
func (o Observation) Healthy() bool {
	return o.Desired > 0 && o.Available >= o.Desired && o.Updated >= o.Desired && o.ObservedGeneration >= o.Generation
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
		if n == "" || n == "default" || n == "kube-system" || n == "resilienceplane" {
			return fmt.Errorf("%w: protected namespace", ErrInvalid)
		}
	}
	return nil
}
func DefaultPolicy() Policy {
	return Policy{Namespaces: []string{"rp-demo"}, MaxReplicas: 3, MaxConcurrent: 1, FreshnessSeconds: 30, DetectionSeconds: 30, DryRun: true}
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
