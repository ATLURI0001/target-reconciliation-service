package dispatch

import "fmt"

// ValidateTargetHealth enforces allowed health status values.
func ValidateTargetHealth(status string) error {
	switch status {
	case "healthy", "degraded", "unhealthy":
		return nil
	default:
		return fmt.Errorf("contract violation: unknown target health %q", status)
	}
}
