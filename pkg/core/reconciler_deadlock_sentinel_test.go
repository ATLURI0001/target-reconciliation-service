package core

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestReconcilerDeadlockSentinel_BasicAcquireAndRelease(t *testing.T) {
	sentinel := NewReconcilerDeadlockSentinel(10 * time.Second)

	err := sentinel.AcquireLocks("worker-1", []string{"target-b", "target-a"}, 1*time.Second)
	if err != nil {
		t.Fatalf("unexpected error acquiring locks: %v", err)
	}

	// Verify both targets held by worker-1
	sentinel.mu.Lock()
	if sentinel.locks["target-a"].holderWorkerID != "worker-1" || sentinel.locks["target-b"].holderWorkerID != "worker-1" {
		t.Fatalf("expected worker-1 to hold target-a and target-b")
	}
	sentinel.mu.Unlock()

	// Release locks
	sentinel.ReleaseLocks("worker-1", []string{"target-a", "target-b"})

	sentinel.mu.Lock()
	if len(sentinel.locks) != 0 {
		t.Fatalf("expected all locks to be released, got %d", len(sentinel.locks))
	}
	sentinel.mu.Unlock()
}

func TestReconcilerDeadlockSentinel_CycleDetection(t *testing.T) {
	sentinel := NewReconcilerDeadlockSentinel(10 * time.Second)

	// Worker 1 holds T1 and is waiting for T2
	sentinel.mu.Lock()
	sentinel.locks["T1"] = &lockEntry{holderWorkerID: "worker-1", acquiredAt: time.Now()}
	sentinel.waitFor["worker-1"] = "T2"

	// Worker 2 holds T2 and attempts to acquire T1 -> cycle detected!
	sentinel.locks["T2"] = &lockEntry{holderWorkerID: "worker-2", acquiredAt: time.Now()}
	sentinel.mu.Unlock()

	err := sentinel.AcquireLocks("worker-2", []string{"T1"}, 50*time.Millisecond)
	if err == nil {
		t.Fatalf("expected deadlock error, got nil")
	}

	var deadlockErr *ReconcilerDeadlockError
	if !errors.As(err, &deadlockErr) {
		t.Fatalf("expected ReconcilerDeadlockError, got %v", err)
	}
	if deadlockErr.IncidentCode != "ERR_INC_5521_CYCLIC_WAIT_DETECTED" {
		t.Fatalf("expected incident code ERR_INC_5521_CYCLIC_WAIT_DETECTED, got %s", deadlockErr.IncidentCode)
	}
}

func TestReconcilerDeadlockSentinel_Timeout(t *testing.T) {
	sentinel := NewReconcilerDeadlockSentinel(10 * time.Second)

	// Worker 1 holds target-1
	err1 := sentinel.AcquireLocks("worker-1", []string{"target-1"}, 1*time.Second)
	if err1 != nil {
		t.Fatalf("worker-1 acquire failed: %v", err1)
	}

	// Worker 2 attempts to acquire target-1 with 30ms timeout
	err2 := sentinel.AcquireLocks("worker-2", []string{"target-1"}, 30*time.Millisecond)
	if err2 == nil {
		t.Fatalf("expected timeout error for worker-2")
	}

	var deadlockErr *ReconcilerDeadlockError
	if !errors.As(err2, &deadlockErr) {
		t.Fatalf("expected ReconcilerDeadlockError, got %v", err2)
	}
	if deadlockErr.IncidentCode != "ERR_INC_5521_LOCK_TIMEOUT" {
		t.Fatalf("expected ERR_INC_5521_LOCK_TIMEOUT, got %s", deadlockErr.IncidentCode)
	}
}

func TestReconcilerDeadlockSentinel_StaleLockReclamation(t *testing.T) {
	sentinel := NewReconcilerDeadlockSentinel(50 * time.Millisecond) // short TTL

	sentinel.mu.Lock()
	sentinel.locks["stale-target"] = &lockEntry{
		holderWorkerID: "crashed-worker",
		acquiredAt:     time.Now().Add(-100 * time.Millisecond), // expired
	}
	sentinel.mu.Unlock()

	// New worker acquiring the stale target succeeds by auto-reclaiming it
	err := sentinel.AcquireLocks("worker-new", []string{"stale-target"}, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("expected acquire to succeed after auto-reclaiming stale lock, got: %v", err)
	}

	sentinel.mu.Lock()
	if sentinel.locks["stale-target"].holderWorkerID != "worker-new" {
		t.Fatalf("expected worker-new to hold stale-target")
	}
	sentinel.mu.Unlock()
}

func TestReconcilerDeadlockSentinel_ConcurrentDisjointAccess(t *testing.T) {
	sentinel := NewReconcilerDeadlockSentinel(5 * time.Second)
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		workerIdx := i
		go func() {
			defer wg.Done()
			wID := "worker-" + string(rune('A'+workerIdx))
			target := "target-" + string(rune('A'+workerIdx))

			err := sentinel.AcquireLocks(wID, []string{target}, 1*time.Second)
			if err != nil {
				t.Errorf("worker %s failed to acquire: %v", wID, err)
				return
			}
			time.Sleep(10 * time.Millisecond)
			sentinel.ReleaseLocks(wID, []string{target})
		}()
	}
	wg.Wait()
}
