package dispatch

import (
	"testing"
	"time"
)

func TestAssertFreshLease(t *testing.T) {
	now := time.Unix(1000, 0)
	if err := AssertFreshLease(now.Add(10*time.Second), now); err != nil {
		t.Fatalf("expected fresh lease, got %v", err)
	}
	if err := AssertFreshLease(now.Add(-1*time.Second), now); err == nil {
		t.Fatal("expected stale lock sentinel violation")
	}
}
