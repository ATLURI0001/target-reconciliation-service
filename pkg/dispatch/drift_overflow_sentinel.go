package dispatch

import "fmt"

// CheckDriftLimit ensures reconciliation drift count does not exceed maxAllowed.
func CheckDriftLimit(driftCount, maxAllowed int) error {
	if driftCount < 0 {
		return fmt.Errorf("sentinel violation: negative drift count %d", driftCount)
	}
	if driftCount > maxAllowed {
		return fmt.Errorf("sentinel violation: drift %d exceeds limit %d", driftCount, maxAllowed)
	}
	return nil
}
