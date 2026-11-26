package dispatch

import "fmt"

// ValidateStateTransition ensures target state version strictly increments.
func ValidateStateTransition(currentVersion, nextVersion int64) error {
	if nextVersion != currentVersion+1 {
		return fmt.Errorf("contract violation: expected version %d, got %d", currentVersion+1, nextVersion)
	}
	return nil
}
