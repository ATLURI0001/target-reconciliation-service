package core

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	ErrProtectedResourceModification = errors.New("security guard: modification or deletion of protected system resource is prohibited")
	ErrDestructionBlastRadiusExceeded = errors.New("security guard: proposed deletion count exceeds blast radius threshold")
	ErrReconciliationDigestMismatch  = errors.New("security guard: cryptographic state hash mismatch indicates manifest tampering")
	ErrMonotonicVersionRollback      = errors.New("security guard: target generation version is not strictly greater than applied version")
)

// ReconciliationResource represents an item to be verified.
type ReconciliationResource struct {
	ResourceID string            `json:"resource_id"`
	Kind       string            `json:"kind"`
	Version    int64             `json:"version"`
	StateHash  string            `json:"state_hash"`
	Attributes map[string]string `json:"attributes"`
}

// DiffAction represents the operation to perform.
type DiffAction string

const (
	ActionCreate DiffAction = "CREATE"
	ActionUpdate DiffAction = "UPDATE"
	ActionDelete DiffAction = "DELETE"
)

// ResourcePlanItem represents a proposed reconciliation action.
type ResourcePlanItem struct {
	Action   DiffAction             `json:"action"`
	Resource ReconciliationResource `json:"resource"`
}

// HashGuardConfig configures security thresholds.
type HashGuardConfig struct {
	ProtectedPrefixes          []string
	MaxDestructionThresholdPct float64 // e.g. 20.0 = max 20% deletion in single run
	MinResourcesForBlastRadius int
}

// DefaultHashGuardConfig returns baseline defensive settings.
func DefaultHashGuardConfig() HashGuardConfig {
	return HashGuardConfig{
		ProtectedPrefixes: []string{
			"system/",
			"iam/",
			"security/",
			"kube-system/",
		},
		MaxDestructionThresholdPct: 20.0,
		MinResourcesForBlastRadius: 5,
	}
}

// ReconciliationHashGuard validates state integrity, blast radius, and protected resources.
type ReconciliationHashGuard struct {
	mu     sync.RWMutex
	cfg    HashGuardConfig
	appliedVersions map[string]int64
}

// NewReconciliationHashGuard constructs a new guard instance.
func NewReconciliationHashGuard(cfg HashGuardConfig) *ReconciliationHashGuard {
	if cfg.MaxDestructionThresholdPct <= 0 || cfg.MaxDestructionThresholdPct > 100 {
		cfg.MaxDestructionThresholdPct = 20.0
	}
	if len(cfg.ProtectedPrefixes) == 0 {
		cfg.ProtectedPrefixes = DefaultHashGuardConfig().ProtectedPrefixes
	}
	if cfg.MinResourcesForBlastRadius <= 0 {
		cfg.MinResourcesForBlastRadius = 5
	}

	return &ReconciliationHashGuard{
		cfg:             cfg,
		appliedVersions: make(map[string]int64),
	}
}

// ComputeDeterministicStateHash calculates a canonical SHA-256 digest across resource fields.
func ComputeDeterministicStateHash(res ReconciliationResource) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s|%s|%d|", res.ResourceID, res.Kind, res.Version)))

	// Sort attribute keys deterministically
	keys := make([]string, 0, len(res.Attributes))
	for k := range res.Attributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		h.Write([]byte(fmt.Sprintf("%s=%s;", k, res.Attributes[k])))
	}

	return hex.EncodeToString(h.Sum(nil))
}

// ValidateReconciliationPlan checks for protected resource violations, blast radius ceilings,
// monotonic version ordering, and cryptographic digest tampering.
func (g *ReconciliationHashGuard) ValidateReconciliationPlan(
	totalActiveCount int,
	plan []ResourcePlanItem,
	expectedPlanDigest string,
) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	deleteCount := 0
	hasher := sha256.New()

	for _, item := range plan {
		res := item.Resource
		rID := strings.TrimSpace(res.ResourceID)

		// 1. Protected resource check
		for _, prefix := range g.cfg.ProtectedPrefixes {
			if strings.HasPrefix(rID, prefix) {
				if item.Action == ActionDelete || item.Action == ActionUpdate {
					return fmt.Errorf("%w: cannot modify or delete %q", ErrProtectedResourceModification, rID)
				}
			}
		}

		// 2. Monotonic version ordering check
		if appliedVer, exists := g.appliedVersions[rID]; exists {
			if res.Version <= appliedVer && item.Action != ActionDelete {
				return fmt.Errorf("%w: resource %q incoming version %d <= applied version %d",
					ErrMonotonicVersionRollback, rID, res.Version, appliedVer)
			}
		}

		// 3. Cryptographic state hash validation
		expectedHash := ComputeDeterministicStateHash(res)
		if res.StateHash != "" && res.StateHash != expectedHash {
			return fmt.Errorf("%w: resource %q payload has been modified", ErrReconciliationDigestMismatch, rID)
		}

		if item.Action == ActionDelete {
			deleteCount++
		}

		hasher.Write([]byte(fmt.Sprintf("%s:%s:%s;", item.Action, rID, expectedHash)))
	}

	// 4. Destruction blast radius protection
	if totalActiveCount >= g.cfg.MinResourcesForBlastRadius {
		delPct := (float64(deleteCount) / float64(totalActiveCount)) * 100.0
		if delPct > g.cfg.MaxDestructionThresholdPct {
			return fmt.Errorf("%w: attempted to delete %d of %d resources (%.1f%% > %.1f%% threshold)",
				ErrDestructionBlastRadiusExceeded, deleteCount, totalActiveCount, delPct, g.cfg.MaxDestructionThresholdPct)
		}
	}

	// 5. Plan-level digest verification (if supplied)
	computedPlanDigest := hex.EncodeToString(hasher.Sum(nil))
	if expectedPlanDigest != "" && expectedPlanDigest != computedPlanDigest {
		return fmt.Errorf("%w: plan level digest mismatch", ErrReconciliationDigestMismatch)
	}

	// Commit applied versions
	for _, item := range plan {
		if item.Action == ActionDelete {
			delete(g.appliedVersions, item.Resource.ResourceID)
		} else {
			g.appliedVersions[item.Resource.ResourceID] = item.Resource.Version
		}
	}

	return nil
}
