package core

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var (
	ErrCyclicWaitDeadlock     = errors.New("circular lock dependency detected across reconciliation targets")
	ErrLockTimeout            = errors.New("lock acquisition deadline exceeded")
	ErrInvalidWorkerOrTargets = errors.New("workerID and targetIDs must be non-empty")
)

// ReconcilerDeadlockError provides structured error reporting for INC-2026-RECON-5521.
type ReconcilerDeadlockError struct {
	IncidentCode string
	Message      string
	WorkerID     string
	Targets      []string
}

func (e *ReconcilerDeadlockError) Error() string {
	return fmt.Sprintf("[INC-2026-RECON-5521][%s] %s (worker: %s, targets: %v)", e.IncidentCode, e.Message, e.WorkerID, e.Targets)
}

type lockEntry struct {
	holderWorkerID string
	acquiredAt     time.Time
}

// ReconcilerDeadlockSentinel prevents circular wait deadlocks and lock contention during reconciliations.
type ReconcilerDeadlockSentinel struct {
	mu            sync.Mutex
	locks         map[string]*lockEntry  // targetID -> lockEntry
	waitFor       map[string]string      // workerID -> targetID waiting on
	heldBy        map[string][]string    // workerID -> list of targetIDs held
	maxLockHoldMs time.Duration
}

// NewReconcilerDeadlockSentinel creates a deadlock sentinel instance.
func NewReconcilerDeadlockSentinel(maxLockHold time.Duration) *ReconcilerDeadlockSentinel {
	if maxLockHold <= 0 {
		maxLockHold = 30 * time.Second
	}
	return &ReconcilerDeadlockSentinel{
		locks:         make(map[string]*lockEntry),
		waitFor:       make(map[string]string),
		heldBy:        make(map[string][]string),
		maxLockHoldMs: maxLockHold,
	}
}

// detectCycle checks whether adding edge workerID -> waitingOnTarget creates a cycle in wait-for graph.
// Must be called with sentinel.mu held.
func (s *ReconcilerDeadlockSentinel) detectCycle(workerID, waitingOnTarget string) bool {
	visitedWorkers := make(map[string]bool)
	currentWorker := workerID

	for {
		visitedWorkers[currentWorker] = true
		entry, exists := s.locks[waitingOnTarget]
		if !exists || entry.holderWorkerID == "" {
			return false
		}

		holder := entry.holderWorkerID
		if holder == workerID {
			// Found cycle back to originating worker!
			return true
		}
		if visitedWorkers[holder] {
			return true
		}

		// What is the holder waiting on?
		nextTarget, isWaiting := s.waitFor[holder]
		if !isWaiting {
			return false
		}
		currentWorker = holder
		waitingOnTarget = nextTarget
	}
}

// AcquireLocks atomically acquires all specified target locks in canonical sorted order,
// blocking circular wait cycles before they occur.
func (s *ReconcilerDeadlockSentinel) AcquireLocks(workerID string, targetIDs []string, timeout time.Duration) error {
	if workerID == "" || len(targetIDs) == 0 {
		return ErrInvalidWorkerOrTargets
	}

	// 1. Enforce canonical ordering (lexicographical sorting) to eliminate inverted acquisition deadlocks
	sortedTargets := make([]string, len(targetIDs))
	copy(sortedTargets, targetIDs)
	sort.Strings(sortedTargets)

	// Deduplicate targets
	dedup := make([]string, 0, len(sortedTargets))
	for i, t := range sortedTargets {
		if i == 0 || t != sortedTargets[i-1] {
			dedup = append(dedup, t)
		}
	}

	deadline := time.Now().Add(timeout)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean up any stale locks first
	s.reclaimStaleLocksLocked(time.Now())

	// 2. Pre-flight cycle detection across all requested targets
	for _, target := range dedup {
		if entry, exists := s.locks[target]; exists && entry.holderWorkerID != workerID {
			if s.detectCycle(workerID, target) {
				return &ReconcilerDeadlockError{
					IncidentCode: "ERR_INC_5521_CYCLIC_WAIT_DETECTED",
					Message:      "Circular lock dependency detected in wait-for graph",
					WorkerID:     workerID,
					Targets:      dedup,
				}
			}
		}
	}

	// 3. Acquire all targets in sorted order
	acquiredThisCall := make([]string, 0, len(dedup))

	for _, target := range dedup {
		for {
			entry, exists := s.locks[target]
			if !exists || entry.holderWorkerID == "" || entry.holderWorkerID == workerID {
				// Target is free or already owned by worker
				s.locks[target] = &lockEntry{
					holderWorkerID: workerID,
					acquiredAt:     time.Now(),
				}
				acquiredThisCall = append(acquiredThisCall, target)
				break
			}

			// Lock currently held by another worker
			s.waitFor[workerID] = target

			// Check cycle again under lock
			if s.detectCycle(workerID, target) {
				delete(s.waitFor, workerID)
				s.rollbackAcquisitionsLocked(workerID, acquiredThisCall)
				return &ReconcilerDeadlockError{
					IncidentCode: "ERR_INC_5521_CYCLIC_WAIT_DETECTED",
					Message:      "Circular lock dependency detected while waiting",
					WorkerID:     workerID,
					Targets:      dedup,
				}
			}

			if time.Now().After(deadline) {
				delete(s.waitFor, workerID)
				s.rollbackAcquisitionsLocked(workerID, acquiredThisCall)
				return &ReconcilerDeadlockError{
					IncidentCode: "ERR_INC_5521_LOCK_TIMEOUT",
					Message:      fmt.Sprintf("Timeout waiting for target lock '%s'", target),
					WorkerID:     workerID,
					Targets:      dedup,
				}
			}

			// Release lock temporarily and yield
			s.mu.Unlock()
			time.Sleep(5 * time.Millisecond)
			s.mu.Lock()
		}
	}

	delete(s.waitFor, workerID)
	s.heldBy[workerID] = append(s.heldBy[workerID], acquiredThisCall...)
	return nil
}

// ReleaseLocks releases specified locks held by worker.
func (s *ReconcilerDeadlockSentinel) ReleaseLocks(workerID string, targetIDs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, target := range targetIDs {
		if entry, exists := s.locks[target]; exists && entry.holderWorkerID == workerID {
			delete(s.locks, target)
		}
	}

	// Remove from heldBy
	var remaining []string
	releaseMap := make(map[string]bool)
	for _, t := range targetIDs {
		releaseMap[t] = true
	}
	for _, t := range s.heldBy[workerID] {
		if !releaseMap[t] {
			remaining = append(remaining, t)
		}
	}
	if len(remaining) == 0 {
		delete(s.heldBy, workerID)
	} else {
		s.heldBy[workerID] = remaining
	}
	delete(s.waitFor, workerID)
}

func (s *ReconcilerDeadlockSentinel) rollbackAcquisitionsLocked(workerID string, targets []string) {
	for _, t := range targets {
		if entry, exists := s.locks[t]; exists && entry.holderWorkerID == workerID {
			delete(s.locks, t)
		}
	}
}

func (s *ReconcilerDeadlockSentinel) reclaimStaleLocksLocked(now time.Time) int {
	reclaimed := 0
	for target, entry := range s.locks {
		if now.Sub(entry.acquiredAt) > s.maxLockHoldMs {
			delete(s.locks, target)
			reclaimed++
		}
	}
	return reclaimed
}

// ReclaimStaleLocks reaps expired or orphaned locks.
func (s *ReconcilerDeadlockSentinel) ReclaimStaleLocks() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reclaimStaleLocksLocked(time.Now())
}
