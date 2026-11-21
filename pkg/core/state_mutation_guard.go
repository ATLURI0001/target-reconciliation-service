package core

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrResourceLocked     = errors.New("resource is locked by another active leaseholder")
	ErrLockNotOwned       = errors.New("cannot release or modify lease: lock is not owned by caller")
	ErrLeaseExpired       = errors.New("resource lease has expired")
	ErrGenerationMismatch = errors.New("optimistic concurrency conflict: generation token mismatch")
	ErrMutationNotFound   = errors.New("mutation record not found")
	ErrInvalidMutation    = errors.New("mutation is not in pending state")
	ErrEmptyIdentifier    = errors.New("urn and owner identifiers cannot be empty")
)

// MutationStatus represents the lifecycle phase of an in-flight state mutation.
type MutationStatus string

const (
	StatusPending    MutationStatus = "PENDING"
	StatusCommitted  MutationStatus = "COMMITTED"
	StatusRolledBack MutationStatus = "ROLLED_BACK"
	StatusExpired    MutationStatus = "EXPIRED"
)

// LeaseLock tracks an active distributed lease held on a resource URN.
type LeaseLock struct {
	URN        string    `json:"urn"`
	OwnerID    string    `json:"owner_id"`
	ExpiresAt  time.Time `json:"expires_at"`
	Generation uint64    `json:"generation"`
}

// IsExpired checks if the lease has elapsed relative to now.
func (l *LeaseLock) IsExpired() bool {
	return time.Now().After(l.ExpiresAt)
}

// MutationRecord details a tracked state modification attempt.
type MutationRecord struct {
	MutationID  string         `json:"mutation_id"`
	URN         string         `json:"urn"`
	OwnerID     string         `json:"owner_id"`
	ExpectedGen uint64         `json:"expected_gen"`
	NewGen      uint64         `json:"new_gen"`
	Status      MutationStatus `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
}

// StateMutationGuard orchestrates atomic lease locking, optimistic concurrency, and mutation lifecycles.
type StateMutationGuard struct {
	mu          sync.RWMutex
	leases      map[string]*LeaseLock       // urn -> lease
	generations map[string]uint64           // urn -> current generation
	mutations   map[string]*MutationRecord  // mutationID -> record
	mutationSeq uint64
}

// NewStateMutationGuard constructs a clean mutation guard instance.
func NewStateMutationGuard() *StateMutationGuard {
	return &StateMutationGuard{
		leases:      make(map[string]*LeaseLock),
		generations: make(map[string]uint64),
		mutations:   make(map[string]*MutationRecord),
	}
}

// AcquireLease attempts to obtain an exclusive lease for a given URN.
// If the caller already owns the lease, this extends the lease TTL (reentrant extension).
func (g *StateMutationGuard) AcquireLease(urn, ownerID string, ttl time.Duration) (*LeaseLock, error) {
	if urn == "" || ownerID == "" {
		return nil, ErrEmptyIdentifier
	}
	if ttl <= 0 {
		ttl = 5 * time.Second
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	existing, exists := g.leases[urn]

	if exists && !now.After(existing.ExpiresAt) {
		// Lease is currently active
		if existing.OwnerID == ownerID {
			// Reentrant lease extension
			existing.ExpiresAt = now.Add(ttl)
			return cloneLease(existing), nil
		}
		return nil, fmt.Errorf("%w: current owner %s holds lease until %s", ErrResourceLocked, existing.OwnerID, existing.ExpiresAt)
	}

	// Create or reassign lease
	currentGen := g.generations[urn]
	lease := &LeaseLock{
		URN:        urn,
		OwnerID:    ownerID,
		ExpiresAt:  now.Add(ttl),
		Generation: currentGen,
	}
	g.leases[urn] = lease

	return cloneLease(lease), nil
}

// ReleaseLease explicitly yields a held lease before expiration.
func (g *StateMutationGuard) ReleaseLease(urn, ownerID string) error {
	if urn == "" || ownerID == "" {
		return ErrEmptyIdentifier
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	existing, exists := g.leases[urn]
	if !exists {
		return nil // already clean
	}

	if existing.OwnerID != ownerID {
		return fmt.Errorf("%w: owned by %s", ErrLockNotOwned, existing.OwnerID)
	}

	delete(g.leases, urn)
	return nil
}

// BeginMutation creates a tracked pending mutation under optimistic concurrency check.
func (g *StateMutationGuard) BeginMutation(urn, ownerID string, expectedGen uint64) (*MutationRecord, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	existing, exists := g.leases[urn]
	if !exists || time.Now().After(existing.ExpiresAt) {
		return nil, ErrLeaseExpired
	}
	if existing.OwnerID != ownerID {
		return nil, ErrLockNotOwned
	}

	currentGen := g.generations[urn]
	if expectedGen != currentGen {
		return nil, fmt.Errorf("%w: expected %d, current %d", ErrGenerationMismatch, expectedGen, currentGen)
	}

	g.mutationSeq++
	mutID := fmt.Sprintf("mut-%s-%d", urn, g.mutationSeq)
	rec := &MutationRecord{
		MutationID:  mutID,
		URN:         urn,
		OwnerID:     ownerID,
		ExpectedGen: expectedGen,
		NewGen:      expectedGen + 1,
		Status:      StatusPending,
		CreatedAt:   time.Now(),
	}
	g.mutations[mutID] = rec

	return cloneRecord(rec), nil
}

// CommitMutation applies a pending state change, incrementing generation count.
func (g *StateMutationGuard) CommitMutation(mutationID string) (*MutationRecord, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	rec, exists := g.mutations[mutationID]
	if !exists {
		return nil, ErrMutationNotFound
	}
	if rec.Status != StatusPending {
		return nil, fmt.Errorf("%w: status is %s", ErrInvalidMutation, rec.Status)
	}

	// Advance generation
	g.generations[rec.URN] = rec.NewGen
	if lease, ok := g.leases[rec.URN]; ok {
		lease.Generation = rec.NewGen
	}

	rec.Status = StatusCommitted
	return cloneRecord(rec), nil
}

// RollbackMutation aborts a pending state mutation without altering the generation counter.
func (g *StateMutationGuard) RollbackMutation(mutationID string) (*MutationRecord, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	rec, exists := g.mutations[mutationID]
	if !exists {
		return nil, ErrMutationNotFound
	}
	if rec.Status != StatusPending {
		return nil, fmt.Errorf("%w: status is %s", ErrInvalidMutation, rec.Status)
	}

	rec.Status = StatusRolledBack
	return cloneRecord(rec), nil
}

// PurgeExpiredLeases removes expired lease locks from memory.
func (g *StateMutationGuard) PurgeExpiredLeases() int {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	purged := 0
	for urn, lease := range g.leases {
		if now.After(lease.ExpiresAt) {
			delete(g.leases, urn)
			purged++
		}
	}
	return purged
}

// GetResourceGeneration returns the current monotonic generation token for a URN.
func (g *StateMutationGuard) GetResourceGeneration(urn string) uint64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.generations[urn]
}

// ActiveLeaseCount returns the number of unexpired leases currently tracked.
func (g *StateMutationGuard) ActiveLeaseCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	now := time.Now()
	active := 0
	for _, lease := range g.leases {
		if !now.After(lease.ExpiresAt) {
			active++
		}
	}
	return active
}

func cloneLease(l *LeaseLock) *LeaseLock {
	return &LeaseLock{
		URN:        l.URN,
		OwnerID:    l.OwnerID,
		ExpiresAt:  l.ExpiresAt,
		Generation: l.Generation,
	}
}

func cloneRecord(r *MutationRecord) *MutationRecord {
	return &MutationRecord{
		MutationID:  r.MutationID,
		URN:         r.URN,
		OwnerID:     r.OwnerID,
		ExpectedGen: r.ExpectedGen,
		NewGen:      r.NewGen,
		Status:      r.Status,
		CreatedAt:   r.CreatedAt,
	}
}
