package core

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrMissingIdempotencyKey   = errors.New("idempotency: key cannot be empty")
	ErrIdempotencyHashMismatch = errors.New("idempotency: key reused with conflicting payload digest")
	ErrIdempotencyInFlight     = errors.New("idempotency: operation with this key is currently in flight")
)

type idempotencyRecord struct {
	PayloadHash string
	Completed   bool
	ExpiresAt   time.Time
}

// IdempotencyContract enforces exactly-once reconciliation execution invariants.
type IdempotencyContract struct {
	mu      sync.Mutex
	ttl     time.Duration
	records map[string]idempotencyRecord
}

// NewIdempotencyContract creates an idempotency contract enforcer with TTL.
func NewIdempotencyContract(ttl time.Duration) *IdempotencyContract {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &IdempotencyContract{
		ttl:     ttl,
		records: make(map[string]idempotencyRecord),
	}
}

// Acquire checks and reserves an idempotency key for a payload hash.
// Returns (alreadyCompleted, error).
func (c *IdempotencyContract) Acquire(key string, payloadHash string, now time.Time) (bool, error) {
	if key == "" {
		return false, ErrMissingIdempotencyKey
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if rec, exists := c.records[key]; exists {
		if now.Before(rec.ExpiresAt) {
			if rec.PayloadHash != payloadHash {
				return false, ErrIdempotencyHashMismatch
			}
			if !rec.Completed {
				return false, ErrIdempotencyInFlight
			}
			return true, nil // Already completed safely
		}
	}

	c.records[key] = idempotencyRecord{
		PayloadHash: payloadHash,
		Completed:   false,
		ExpiresAt:   now.Add(c.ttl),
	}
	return false, nil
}

// Complete marks an in-flight idempotency key as completed.
func (c *IdempotencyContract) Complete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if rec, exists := c.records[key]; exists {
		rec.Completed = true
		c.records[key] = rec
	}
}
