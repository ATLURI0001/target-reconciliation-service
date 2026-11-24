package core

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalidPlanID         = errors.New("invalid plan_id format or length")
	ErrDuplicateResourceDiff = errors.New("duplicate action targeting the same resource_id in plan")
	ErrUnsupportedAction     = errors.New("unsupported diff action type")
	ErrInvalidResourceID     = errors.New("invalid resource_id format or length")
	ErrEmptyDiffPlan         = errors.New("diff plan contains no actions")
	ErrMaxPlanSizeExceeded   = errors.New("diff plan exceeds maximum allowed actions count")
	ErrBlastRadiusExceeded   = errors.New("plan exceeds destructive deletion blast radius safety limit")
)

var (
	planIDRegex     = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]{3,128}$`)
	resourceIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-\.\/]{1,256}$`)
)

// ActionNoop extends DiffAction with the no-op state.
const (
	ActionNoop DiffAction = "NOOP"
)

// PlanSafetyLimits configures defensive bounds on reconciliation execution.
type PlanSafetyLimits struct {
	MaxActionsPerPlan   int     `json:"max_actions_per_plan"`
	MaxDeletionsPerPlan int     `json:"max_deletions_per_plan"`
	MaxDeletionRatio    float64 `json:"max_deletion_ratio"` // e.g. 0.40 -> 40%
	MaxDetailsLength    int     `json:"max_details_length"`
	AllowEmptyPlan      bool    `json:"allow_empty_plan"`
}

// DefaultPlanSafetyLimits provides resilient production defaults.
func DefaultPlanSafetyLimits() PlanSafetyLimits {
	return PlanSafetyLimits{
		MaxActionsPerPlan:   5000,
		MaxDeletionsPerPlan: 200,
		MaxDeletionRatio:    0.35, // Reject if deletions exceed 35% of non-noop resources
		MaxDetailsLength:    4096,
		AllowEmptyPlan:      false,
	}
}

// PlanAuditSummary provides metadata extracted during contract validation.
type PlanAuditSummary struct {
	PlanID            string         `json:"plan_id"`
	TotalActions      int            `json:"total_actions"`
	ActionCounts      map[string]int `json:"action_counts"`
	DestructiveRatio  float64        `json:"destructive_ratio"`
	ValidatedAt       time.Time      `json:"validated_at"`
	HasDestructiveOps bool           `json:"has_destructive_ops"`
}

// ReconciliationPlanContract validates and hardens planned reconciliation execution.
type ReconciliationPlanContract struct {
	limits PlanSafetyLimits
}

// NewReconciliationPlanContract constructs a plan validator with limits.
func NewReconciliationPlanContract(limits PlanSafetyLimits) *ReconciliationPlanContract {
	if limits.MaxActionsPerPlan <= 0 {
		limits.MaxActionsPerPlan = 5000
	}
	if limits.MaxDeletionsPerPlan <= 0 {
		limits.MaxDeletionsPerPlan = 200
	}
	if limits.MaxDeletionRatio <= 0.0 || limits.MaxDeletionRatio > 1.0 {
		limits.MaxDeletionRatio = 0.35
	}
	if limits.MaxDetailsLength <= 0 {
		limits.MaxDetailsLength = 4096
	}
	return &ReconciliationPlanContract{limits: limits}
}

// ValidateAndSanitize inspects an ingress DiffPlan, asserting mathematical and schema invariants.
func (c *ReconciliationPlanContract) ValidateAndSanitize(plan DiffPlan) (DiffPlan, PlanAuditSummary, error) {
	cleanPlanID := strings.TrimSpace(plan.PlanID)
	if !planIDRegex.MatchString(cleanPlanID) {
		return DiffPlan{}, PlanAuditSummary{}, fmt.Errorf("%w: '%s'", ErrInvalidPlanID, cleanPlanID)
	}

	numActions := len(plan.Diffs)
	if numActions == 0 && !c.limits.AllowEmptyPlan {
		return DiffPlan{}, PlanAuditSummary{}, ErrEmptyDiffPlan
	}

	if numActions > c.limits.MaxActionsPerPlan {
		return DiffPlan{}, PlanAuditSummary{}, fmt.Errorf("%w: count %d exceeds max %d", ErrMaxPlanSizeExceeded, numActions, c.limits.MaxActionsPerPlan)
	}

	sanitizedDiffs := make([]ResourceDiff, 0, numActions)
	seenResources := make(map[string]string, numActions)
	actionCounts := make(map[string]int)

	deletionCount := 0
	activeResourceCount := 0

	for i, d := range plan.Diffs {
		resID := strings.TrimSpace(d.ResourceID)
		if !resourceIDRegex.MatchString(resID) {
			return DiffPlan{}, PlanAuditSummary{}, fmt.Errorf("%w at index %d: '%s'", ErrInvalidResourceID, i, resID)
		}

		if prevAction, exists := seenResources[resID]; exists {
			return DiffPlan{}, PlanAuditSummary{}, fmt.Errorf("%w: resource '%s' targeted by multiple actions (%s and %s)", ErrDuplicateResourceDiff, resID, prevAction, d.Action)
		}

		actionUpper := strings.ToUpper(strings.TrimSpace(d.Action))
		switch DiffAction(actionUpper) {
		case ActionCreate, ActionUpdate, ActionDelete, ActionNoop:
			// Valid action
		default:
			return DiffPlan{}, PlanAuditSummary{}, fmt.Errorf("%w: '%s' at index %d", ErrUnsupportedAction, d.Action, i)
		}

		seenResources[resID] = actionUpper
		actionCounts[actionUpper]++

		if DiffAction(actionUpper) == ActionDelete {
			deletionCount++
		}
		if DiffAction(actionUpper) != ActionNoop {
			activeResourceCount++
		}

		cleanDetails := strings.TrimSpace(d.Details)
		if len(cleanDetails) > c.limits.MaxDetailsLength {
			cleanDetails = cleanDetails[:c.limits.MaxDetailsLength]
		}

		sanitizedDiffs = append(sanitizedDiffs, ResourceDiff{
			ResourceID: resID,
			Action:     actionUpper,
			Details:    cleanDetails,
		})
	}

	// Blast radius deletion checks
	if deletionCount > c.limits.MaxDeletionsPerPlan {
		return DiffPlan{}, PlanAuditSummary{}, fmt.Errorf("%w: %d deletions exceeds max allowed %d", ErrBlastRadiusExceeded, deletionCount, c.limits.MaxDeletionsPerPlan)
	}

	var deletionRatio float64
	if activeResourceCount > 5 { // Evaluate ratio only when significant actions exist
		deletionRatio = float64(deletionCount) / float64(activeResourceCount)
		if deletionRatio > c.limits.MaxDeletionRatio {
			return DiffPlan{}, PlanAuditSummary{}, fmt.Errorf("%w: deletion ratio %.2f exceeds safety ceiling %.2f", ErrBlastRadiusExceeded, deletionRatio, c.limits.MaxDeletionRatio)
		}
	}

	sanitizedPlan := DiffPlan{
		PlanID: cleanPlanID,
		Diffs:  sanitizedDiffs,
	}

	summary := PlanAuditSummary{
		PlanID:            cleanPlanID,
		TotalActions:      numActions,
		ActionCounts:      actionCounts,
		DestructiveRatio:  deletionRatio,
		ValidatedAt:       time.Now(),
		HasDestructiveOps: deletionCount > 0,
	}

	return sanitizedPlan, summary, nil
}
