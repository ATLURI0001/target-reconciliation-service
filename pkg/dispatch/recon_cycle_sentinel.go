package dispatch

import "fmt"

// AssertNoDependencyCycle detects self-dependencies in reconciliation targets.
func AssertNoDependencyCycle(targetID string, deps []string) error {
	for _, d := range deps {
		if d == targetID {
			return fmt.Errorf("sentinel violation: target %s depends on itself", targetID)
		}
	}
	return nil
}
