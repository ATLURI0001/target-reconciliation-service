package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	// ErrUnsupportedAPIVersion indicates an invalid or unsupported apiVersion string.
	ErrUnsupportedAPIVersion = errors.New("unsupported manifest apiVersion")
	// ErrInvalidKind indicates an unrecognized manifest kind.
	ErrInvalidKind = errors.New("invalid manifest kind, expected 'TargetManifest'")
	// ErrInvalidMetadata indicates missing or malformed metadata fields.
	ErrInvalidMetadata = errors.New("manifest metadata validation failed")
	// ErrInvalidTargetSpec indicates a constraint violation in the reconciliation spec.
	ErrInvalidTargetSpec = errors.New("manifest spec validation failed")
	// ErrCyclicManifestDependencies indicates circular dependency declarations among manifest resources.
	ErrCyclicManifestDependencies = errors.New("cyclic dependency detected within manifest resource graph")
)

var (
	targetIDRegex    = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	metadataNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
)

// Supported API versions for schema evolution.
const (
	APIVersionV1Alpha1 = "reconciliation.dev/v1alpha1"
	APIVersionV1Beta1  = "reconciliation.dev/v1beta1"
	APIVersionV1       = "reconciliation.dev/v1"
	KindTargetManifest = "TargetManifest"
)

// ReconciliationPolicy specifies the convergence guarantee.
type ReconciliationPolicy string

const (
	PolicyStrictConvergence ReconciliationPolicy = "StrictConvergence"
	PolicyBestEffort        ReconciliationPolicy = "BestEffort"
	PolicyDryRun            ReconciliationPolicy = "DryRun"
	PolicyCanary            ReconciliationPolicy = "Canary"
)

// BackoffType specifies retry backoff algorithms.
type BackoffType string

const (
	BackoffLinear      BackoffType = "linear"
	BackoffExponential BackoffType = "exponential"
)

// ManifestMetadata provides standard object metadata.
type ManifestMetadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Generation  int64             `json:"generation,omitempty"`
	CreatedAt   *time.Time        `json:"createdAt,omitempty"`
}

// ResourceRequirement defines an individual entity requirement in the reconciliation plan.
type ResourceRequirement struct {
	Kind         string   `json:"kind"`
	Name         string   `json:"name"`
	Required     bool     `json:"required"`
	Dependencies []string `json:"dependencies,omitempty"` // List of "Kind/Name" dependencies
}

// RetryPolicy defines error backoff rules.
type RetryPolicy struct {
	MaxRetries      int         `json:"maxRetries"`
	BackoffType     BackoffType `json:"backoffType"`
	BaseIntervalSec int         `json:"baseIntervalSec"`
	MaxIntervalSec  int         `json:"maxIntervalSec"`
}

// TargetManifestSpec describes the desired state configuration and convergence constraints.
type TargetManifestSpec struct {
	TargetID                    string                `json:"targetId"`
	ReconciliationPolicy        ReconciliationPolicy  `json:"reconciliationPolicy"`
	SyncIntervalSec             int                   `json:"syncIntervalSec"`
	ConcurrencyLimit            int                   `json:"concurrencyLimit"`
	RetryPolicy                 RetryPolicy           `json:"retryPolicy"`
	Resources                   []ResourceRequirement `json:"resources"`
	DegradationThresholdPercent float64               `json:"degradationThresholdPercent"`
}

// TargetManifest is the unified admission entity for declarative reconciliation goals.
type TargetManifest struct {
	APIVersion string             `json:"apiVersion"`
	Kind       string             `json:"kind"`
	Metadata   ManifestMetadata   `json:"metadata"`
	Spec       TargetManifestSpec `json:"spec"`
	SpecHash   string             `json:"specHash,omitempty"`
}

// ParseAndSanitizeManifest validates raw JSON against schema requirements and auto-evolves legacy API versions.
func ParseAndSanitizeManifest(rawJSON []byte) (*TargetManifest, error) {
	if len(rawJSON) == 0 {
		return nil, errors.New("empty manifest payload")
	}

	var m TargetManifest
	if err := json.Unmarshal(rawJSON, &m); err != nil {
		return nil, fmt.Errorf("manifest unmarshal error: %w", err)
	}

	// Schema evolution & API version validation
	switch m.APIVersion {
	case APIVersionV1Alpha1, APIVersionV1Beta1:
		// Automatically promote legacy API versions to canonical v1
		m.APIVersion = APIVersionV1
	case APIVersionV1:
		// Canonical version
	default:
		return nil, fmt.Errorf("%w: '%s'", ErrUnsupportedAPIVersion, m.APIVersion)
	}

	// Validate Kind
	if m.Kind != KindTargetManifest {
		return nil, fmt.Errorf("%w: got '%s'", ErrInvalidKind, m.Kind)
	}

	// Validate Metadata
	m.Metadata.Name = strings.TrimSpace(m.Metadata.Name)
	if m.Metadata.Name == "" || !metadataNameRegex.MatchString(m.Metadata.Name) {
		return nil, fmt.Errorf("%w: invalid metadata.name '%s'", ErrInvalidMetadata, m.Metadata.Name)
	}
	if len(m.Metadata.Name) > 253 {
		return nil, fmt.Errorf("%w: metadata.name exceeds 253 chars", ErrInvalidMetadata)
	}
	if m.Metadata.Namespace == "" {
		m.Metadata.Namespace = "default"
	}

	// Validate Spec
	m.Spec.TargetID = strings.TrimSpace(m.Spec.TargetID)
	if m.Spec.TargetID == "" || !targetIDRegex.MatchString(m.Spec.TargetID) {
		return nil, fmt.Errorf("%w: invalid spec.targetId '%s'", ErrInvalidTargetSpec, m.Spec.TargetID)
	}

	// Policy validation
	switch m.Spec.ReconciliationPolicy {
	case PolicyStrictConvergence, PolicyBestEffort, PolicyDryRun, PolicyCanary:
		// Valid
	case "":
		m.Spec.ReconciliationPolicy = PolicyStrictConvergence
	default:
		return nil, fmt.Errorf("%w: invalid reconciliationPolicy '%s'", ErrInvalidTargetSpec, m.Spec.ReconciliationPolicy)
	}

	// SyncInterval clamping
	if m.Spec.SyncIntervalSec <= 0 {
		m.Spec.SyncIntervalSec = 30 // default 30s
	} else if m.Spec.SyncIntervalSec < 5 || m.Spec.SyncIntervalSec > 86400 {
		return nil, fmt.Errorf("%w: syncIntervalSec must be between 5 and 86400", ErrInvalidTargetSpec)
	}

	// Concurrency limit
	if m.Spec.ConcurrencyLimit <= 0 {
		m.Spec.ConcurrencyLimit = 5
	} else if m.Spec.ConcurrencyLimit > 100 {
		return nil, fmt.Errorf("%w: concurrencyLimit cannot exceed 100", ErrInvalidTargetSpec)
	}

	// Retry policy
	if m.Spec.RetryPolicy.MaxRetries < 0 || m.Spec.RetryPolicy.MaxRetries > 20 {
		return nil, fmt.Errorf("%w: maxRetries must be between 0 and 20", ErrInvalidTargetSpec)
	}
	if m.Spec.RetryPolicy.BackoffType == "" {
		m.Spec.RetryPolicy.BackoffType = BackoffExponential
	} else if m.Spec.RetryPolicy.BackoffType != BackoffLinear && m.Spec.RetryPolicy.BackoffType != BackoffExponential {
		return nil, fmt.Errorf("%w: invalid backoffType '%s'", ErrInvalidTargetSpec, m.Spec.RetryPolicy.BackoffType)
	}
	if m.Spec.RetryPolicy.BaseIntervalSec <= 0 {
		m.Spec.RetryPolicy.BaseIntervalSec = 2
	}
	if m.Spec.RetryPolicy.MaxIntervalSec <= 0 {
		m.Spec.RetryPolicy.MaxIntervalSec = 60
	}
	if m.Spec.RetryPolicy.MaxIntervalSec < m.Spec.RetryPolicy.BaseIntervalSec {
		return nil, fmt.Errorf("%w: maxIntervalSec cannot be less than baseIntervalSec", ErrInvalidTargetSpec)
	}

	// Degradation threshold
	if m.Spec.DegradationThresholdPercent < 0 || m.Spec.DegradationThresholdPercent > 100 {
		return nil, fmt.Errorf("%w: degradationThresholdPercent must be in [0, 100]", ErrInvalidTargetSpec)
	}

	// Resources & Dependency Graph Validation
	if len(m.Spec.Resources) == 0 {
		return nil, fmt.Errorf("%w: manifest must define at least one resource", ErrInvalidTargetSpec)
	}

	seenResources := make(map[string]bool)
	for i, r := range m.Spec.Resources {
		r.Kind = strings.TrimSpace(r.Kind)
		r.Name = strings.TrimSpace(r.Name)
		if r.Kind == "" || r.Name == "" {
			return nil, fmt.Errorf("%w: resource #%d has empty kind or name", ErrInvalidTargetSpec, i)
		}
		key := fmt.Sprintf("%s/%s", r.Kind, r.Name)
		if seenResources[key] {
			return nil, fmt.Errorf("%w: duplicate resource definition '%s'", ErrInvalidTargetSpec, key)
		}
		seenResources[key] = true
	}

	// Check dependencies exist and test for circular dependencies
	adj := make(map[string][]string)
	for _, r := range m.Spec.Resources {
		key := fmt.Sprintf("%s/%s", r.Kind, r.Name)
		for _, dep := range r.Dependencies {
			dep = strings.TrimSpace(dep)
			if !seenResources[dep] {
				return nil, fmt.Errorf("%w: resource '%s' depends on unknown resource '%s'", ErrInvalidTargetSpec, key, dep)
			}
			if dep == key {
				return nil, fmt.Errorf("%w: resource '%s' self-depends", ErrCyclicManifestDependencies, key)
			}
			adj[key] = append(adj[key], dep)
		}
	}

	if err := checkCycle(adj); err != nil {
		return nil, err
	}

	// Calculate deterministic SpecHash
	hash, err := ComputeSpecHash(m.Spec)
	if err != nil {
		return nil, fmt.Errorf("failed to compute spec hash: %w", err)
	}
	m.SpecHash = hash

	return &m, nil
}

func checkCycle(adj map[string][]string) error {
	visited := make(map[string]int) // 0: unvisited, 1: visiting, 2: visited

	var dfs func(node string) error
	dfs = func(node string) error {
		visited[node] = 1
		for _, neighbor := range adj[node] {
			if visited[neighbor] == 1 {
				return fmt.Errorf("%w: cycle involving '%s' -> '%s'", ErrCyclicManifestDependencies, node, neighbor)
			}
			if visited[neighbor] == 0 {
				if err := dfs(neighbor); err != nil {
					return err
				}
			}
		}
		visited[node] = 2
		return nil
	}

	for node := range adj {
		if visited[node] == 0 {
			if err := dfs(node); err != nil {
				return err
			}
		}
	}
	return nil
}

// ComputeSpecHash generates a SHA-256 fingerprint over the canonical, sorted spec JSON.
func ComputeSpecHash(spec TargetManifestSpec) (string, error) {
	// Sort resources deterministically by Kind/Name
	sortedRes := make([]ResourceRequirement, len(spec.Resources))
	copy(sortedRes, spec.Resources)
	sort.Slice(sortedRes, func(i, j int) bool {
		ki := fmt.Sprintf("%s/%s", sortedRes[i].Kind, sortedRes[i].Name)
		kj := fmt.Sprintf("%s/%s", sortedRes[j].Kind, sortedRes[j].Name)
		return ki < kj
	})

	for i := range sortedRes {
		sort.Strings(sortedRes[i].Dependencies)
	}
	spec.Resources = sortedRes

	bytes, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}

	h := sha256.Sum256(bytes)
	return hex.EncodeToString(h[:]), nil
}
