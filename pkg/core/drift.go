package core

import (
	"errors"
	"fmt"
)

// DriftType categorizes the nature of state divergence.
type DriftType string

const (
	DriftTypeNone    DriftType = "NONE"
	DriftTypeMissing DriftType = "MISSING" // In desired, missing from actual
	DriftTypeOrphan  DriftType = "ORPHAN"  // In actual, missing from desired
	DriftTypeMutated DriftType = "MUTATED" // Both exist, payload or version diverged
)

// ResourceDrift captures drift telemetry for a single resource.
type ResourceDrift struct {
	ResourceID      string    `json:"resource_id"`
	Type            DriftType `json:"type"`
	DesiredHash     string    `json:"desired_hash,omitempty"`
	ActualHash      string    `json:"actual_hash,omitempty"`
	ObservedVersion int64     `json:"observed_version,omitempty"`
	DesiredVersion  int64     `json:"desired_version,omitempty"`
}

// DriftReport summarizes aggregate cluster divergence.
type DriftReport struct {
	TotalEvaluated int             `json:"total_evaluated"`
	DriftedCount   int             `json:"drifted_count"`
	DriftRatio     float64         `json:"drift_ratio"`
	Severity       string          `json:"severity"` // "CLEAN", "WARNING", "CRITICAL"
	Drifts         []ResourceDrift `json:"drifts"`
}

// DetectStateDrift analyzes a collection of desired and actual resources to detect out-of-band mutations.
func DetectStateDrift(desiredMap, actualMap map[string]*ResourceSpec) (*DriftReport, error) {
	if desiredMap == nil && actualMap == nil {
		return nil, errors.New("desiredMap and actualMap cannot both be nil")
	}

	seen := make(map[string]bool)
	var drifts []ResourceDrift

	// 1. Check all desired resources against actual
	for id, desired := range desiredMap {
		seen[id] = true
		if desired == nil {
			continue
		}

		actual, exists := actualMap[id]
		if !exists || actual == nil {
			drifts = append(drifts, ResourceDrift{
				ResourceID:     id,
				Type:           DriftTypeMissing,
				DesiredHash:    desired.PayloadHash,
				DesiredVersion: desired.Version,
			})
			continue
		}

		// Check for payload mutation or version mismatch
		if desired.PayloadHash != actual.PayloadHash || desired.Version != actual.Version {
			drifts = append(drifts, ResourceDrift{
				ResourceID:      id,
				Type:            DriftTypeMutated,
				DesiredHash:     desired.PayloadHash,
				ActualHash:      actual.PayloadHash,
				DesiredVersion:  desired.Version,
				ObservedVersion: actual.Version,
			})
		}
	}

	// 2. Identify orphaned actual resources not in desired manifest
	for id, actual := range actualMap {
		if seen[id] || actual == nil {
			continue
		}
		drifts = append(drifts, ResourceDrift{
			ResourceID:      id,
			Type:            DriftTypeOrphan,
			ActualHash:      actual.PayloadHash,
			ObservedVersion: actual.Version,
		})
	}

	totalEvaluated := len(seen)
	for id := range actualMap {
		if !seen[id] {
			totalEvaluated++
		}
	}

	driftRatio := 0.0
	if totalEvaluated > 0 {
		driftRatio = float64(len(drifts)) / float64(totalEvaluated)
	}

	severity := "CLEAN"
	if driftRatio > 0.30 || len(drifts) >= 5 {
		severity = "CRITICAL"
	} else if len(drifts) > 0 {
		severity = "WARNING"
	}

	return &DriftReport{
		TotalEvaluated: totalEvaluated,
		DriftedCount:   len(drifts),
		DriftRatio:     driftRatio,
		Severity:       severity,
		Drifts:         drifts,
	}, nil
}

// RollbackStep defines a compensatory action to revert an aborted rollout stage.
type RollbackStep struct {
	ResourceID string `json:"resource_id"`
	Action     string `json:"action"` // "ROLLBACK_DELETE", "ROLLBACK_RESTORE"
	TargetHash string `json:"target_hash,omitempty"`
}

// GenerateCompensatingRollback produces an inverted sequence of actions to safely undo a failed rollout.
func GenerateCompensatingRollback(executedDiffs []ResourceDiff, baseSnapshots map[string]*ResourceSpec) ([]RollbackStep, error) {
	if len(executedDiffs) == 0 {
		return []RollbackStep{}, nil
	}

	// Rollback must happen in reverse order of execution (LIFO)
	steps := make([]RollbackStep, 0, len(executedDiffs))
	for i := len(executedDiffs) - 1; i >= 0; i-- {
		diff := executedDiffs[i]
		switch diff.Action {
		case "CREATE":
			// Newly created resource must be deleted
			steps = append(steps, RollbackStep{
				ResourceID: diff.ResourceID,
				Action:     "ROLLBACK_DELETE",
			})
		case "UPDATE":
			// Revert to base snapshot if available
			base, ok := baseSnapshots[diff.ResourceID]
			if !ok || base == nil {
				return nil, fmt.Errorf("cannot rollback UPDATE on %s: missing base snapshot", diff.ResourceID)
			}
			steps = append(steps, RollbackStep{
				ResourceID: diff.ResourceID,
				Action:     "ROLLBACK_RESTORE",
				TargetHash: base.PayloadHash,
			})
		case "DELETE":
			// Recreate the deleted resource from base snapshot
			base, ok := baseSnapshots[diff.ResourceID]
			if !ok || base == nil {
				return nil, fmt.Errorf("cannot rollback DELETE on %s: missing base snapshot", diff.ResourceID)
			}
			steps = append(steps, RollbackStep{
				ResourceID: diff.ResourceID,
				Action:     "ROLLBACK_RESTORE",
				TargetHash: base.PayloadHash,
			})
		}
	}

	return steps, nil
}
