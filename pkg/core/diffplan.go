package core

// ResourceDiff describes discrepancies between desired and actual state.
type ResourceDiff struct {
	ResourceID string `json:"resource_id"`
	Action     string `json:"action"` // "CREATE", "UPDATE", "DELETE", "NOOP"
	Details    string `json:"details"`
}

// DiffPlan aggregates a batch of planned reconciliation actions.
type DiffPlan struct {
	PlanID  string         `json:"plan_id"`
	Diffs   []ResourceDiff `json:"diffs"`
}

// ActionCounts summarizes operations required to achieve convergence.
func (p DiffPlan) ActionCounts() map[string]int {
	counts := make(map[string]int)
	for _, d := range p.Diffs {
		counts[d.Action]++
	}
	return counts
}

// NeedsAction returns true if any non-NOOP operations are pending.
func (p DiffPlan) NeedsAction() bool {
	for _, d := range p.Diffs {
		if d.Action != "NOOP" {
			return true
		}
	}
	return false
}
