package core

import (
	"sync"
	"testing"
	"time"
)

func TestStateMutationGuard_LeaseLifecycle(t *testing.T) {
	t.Parallel()

	guard := NewStateMutationGuard()
	urn := "urn:service:database:prod-shard-1"
	owner := "worker-alpha"

	// 1. Initial Acquire
	lease, err := guard.AcquireLease(urn, owner, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to acquire lease: %v", err)
	}
	if lease.URN != urn || lease.OwnerID != owner {
		t.Errorf("unexpected lease content: %+v", lease)
	}
	if guard.ActiveLeaseCount() != 1 {
		t.Errorf("expected 1 active lease, got %d", guard.ActiveLeaseCount())
	}

	// 2. Reentrant lease extension by same owner
	extended, err := guard.AcquireLease(urn, owner, 1*time.Second)
	if err != nil {
		t.Fatalf("reentrant lease extension failed: %v", err)
	}
	if !extended.ExpiresAt.After(lease.ExpiresAt) {
		t.Errorf("expected lease expiration to be extended")
	}

	// 3. Competing acquisition by another owner fails
	_, err = guard.AcquireLease(urn, "worker-bravo", 500*time.Millisecond)
	if err == nil {
		t.Fatalf("expected ErrResourceLocked, got nil")
	}

	// 4. Release by unauthorized owner fails
	err = guard.ReleaseLease(urn, "worker-bravo")
	if err == nil {
		t.Fatalf("expected ErrLockNotOwned, got nil")
	}

	// 5. Release by authorized owner succeeds
	err = guard.ReleaseLease(urn, owner)
	if err != nil {
		t.Fatalf("expected clean release, got: %v", err)
	}
	if guard.ActiveLeaseCount() != 0 {
		t.Errorf("expected 0 active leases, got %d", guard.ActiveLeaseCount())
	}
}

func TestStateMutationGuard_OptimisticConcurrencyAndCommit(t *testing.T) {
	t.Parallel()

	guard := NewStateMutationGuard()
	urn := "urn:service:ingress:routing-rule"
	owner := "worker-gamma"

	_, err := guard.AcquireLease(urn, owner, 2*time.Second)
	if err != nil {
		t.Fatalf("acquire lease failed: %v", err)
	}

	gen := guard.GetResourceGeneration(urn)
	if gen != 0 {
		t.Errorf("expected initial gen 0, got %d", gen)
	}

	// Attempt mutation with wrong generation fails
	_, err = guard.BeginMutation(urn, owner, 99)
	if err == nil {
		t.Fatalf("expected generation mismatch error")
	}

	// Valid mutation initiation
	mut, err := guard.BeginMutation(urn, owner, 0)
	if err != nil {
		t.Fatalf("failed to begin mutation: %v", err)
	}
	if mut.Status != StatusPending || mut.NewGen != 1 {
		t.Errorf("unexpected mutation state: %+v", mut)
	}

	// Commit mutation
	committed, err := guard.CommitMutation(mut.MutationID)
	if err != nil {
		t.Fatalf("failed to commit mutation: %v", err)
	}
	if committed.Status != StatusCommitted {
		t.Errorf("expected StatusCommitted, got %s", committed.Status)
	}

	// Verify generation advanced
	newGen := guard.GetResourceGeneration(urn)
	if newGen != 1 {
		t.Errorf("expected generation 1, got %d", newGen)
	}

	// Second commit on already committed record fails
	_, err = guard.CommitMutation(mut.MutationID)
	if err == nil {
		t.Fatalf("expected error on re-committing")
	}
}

func TestStateMutationGuard_Rollback(t *testing.T) {
	t.Parallel()

	guard := NewStateMutationGuard()
	urn := "urn:service:cache:redis-cluster"
	owner := "worker-delta"

	_, _ = guard.AcquireLease(urn, owner, 2*time.Second)

	mut, err := guard.BeginMutation(urn, owner, 0)
	if err != nil {
		t.Fatalf("failed to begin mutation: %v", err)
	}

	rolledBack, err := guard.RollbackMutation(mut.MutationID)
	if err != nil {
		t.Fatalf("failed to rollback: %v", err)
	}
	if rolledBack.Status != StatusRolledBack {
		t.Errorf("expected StatusRolledBack, got %s", rolledBack.Status)
	}

	// Generation remains unchanged
	if guard.GetResourceGeneration(urn) != 0 {
		t.Errorf("expected generation to stay at 0 after rollback")
	}
}

func TestStateMutationGuard_LeaseExpirationAndPurge(t *testing.T) {
	t.Parallel()

	guard := NewStateMutationGuard()
	urn := "urn:service:ephemeral:temp-job"
	owner := "worker-short"

	_, err := guard.AcquireLease(urn, owner, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to acquire lease: %v", err)
	}

	time.Sleep(30 * time.Millisecond)

	// Lease has expired; mutation should be rejected
	_, err = guard.BeginMutation(urn, owner, 0)
	if err == nil {
		t.Fatalf("expected ErrLeaseExpired on elapsed lease")
	}

	// Purge removes it
	purged := guard.PurgeExpiredLeases()
	if purged != 1 {
		t.Errorf("expected 1 purged lease, got %d", purged)
	}

	// Another worker can now re-acquire cleanly
	_, err = guard.AcquireLease(urn, "worker-new", 1*time.Second)
	if err != nil {
		t.Fatalf("expected new worker to acquire lease: %v", err)
	}
}

func TestStateMutationGuard_ConcurrentContention(t *testing.T) {
	t.Parallel()

	guard := NewStateMutationGuard()
	urn := "urn:service:shared:hot-resource"
	numWorkers := 20

	var wg sync.WaitGroup
	var acquiredCount int
	var countMu sync.Mutex

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			owner := string(rune('A' + workerID))
			_, err := guard.AcquireLease(urn, owner, 200*time.Millisecond)
			if err == nil {
				countMu.Lock()
				acquiredCount++
				countMu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if acquiredCount != 1 {
		t.Errorf("expected exactly 1 worker to acquire exclusive lease, got %d", acquiredCount)
	}
}
