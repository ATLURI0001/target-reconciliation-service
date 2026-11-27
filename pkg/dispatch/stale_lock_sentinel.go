package dispatch

import (
	"fmt"
	"time"
)

// AssertFreshLease rejects expired reconciliation lock leases.
func AssertFreshLease(expiresAt time.Time, now time.Time) error {
	if !expiresAt.After(now) {
		return fmt.Errorf("sentinel violation: stale lock lease expired at %v", expiresAt)
	}
	return nil
}
