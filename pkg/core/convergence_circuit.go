package core

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var (
	ErrCircuitOpenFlapping  = errors.New("reconciliation circuit open: resource is flapping rapidly")
	ErrIdempotencyDuplicate = errors.New("reconciliation skipped: identical mutation already in-flight or applied")
	ErrInvalidParameters   = errors.New("invalid circuit parameters: thresholds must be positive")
)

// FlapCircuitState tracks flapping status for a single resource.
type FlapCircuitState string

const (
	CircuitClosed   FlapCircuitState = "CLOSED"    // Normal operation
	CircuitTripped  FlapCircuitState = "TRIPPED"   // Tripped due to rapid oscillation
	CircuitCooling  FlapCircuitState = "COOLING"   // Half-open / cooling down
)

// FlapHistory tracks timestamped state transitions.
type FlapHistory struct {
	ResourceKey     string
	Transitions     []time.Time
	TrippedCount    int
	CooldownUntil   time.Time
	LastAppliedHash string
	State           FlapCircuitState
}

// ConvergenceCircuitConfig holds operational limits.
type ConvergenceCircuitConfig struct {
	FlapWindowSec      int           // Time window to measure reversals (default: 60s)
	MaxFlapsPerWindow  int           // Max flips before circuit trips (default: 3)
	BaseCooldown       time.Duration // Initial freeze time (default: 30s)
	MaxCooldown        time.Duration // Ceiling on exponential backoff (default: 10m)
	IdempotencyTtlSec  int           // Deduplication lease TTL (default: 300s)
}

// ConvergenceCircuit coordinates flap damping and idempotency protection.
type ConvergenceCircuit struct {
	mu      sync.RWMutex
	cfg     ConvergenceCircuitConfig
	records map[string]*FlapHistory
	leases  map[string]time.Time
}

// NewConvergenceCircuit constructs a thread-safe convergence circuit.
func NewConvergenceCircuit(cfg ConvergenceCircuitConfig) (*ConvergenceCircuit, error) {
	if cfg.FlapWindowSec <= 0 || cfg.MaxFlapsPerWindow <= 0 || cfg.BaseCooldown <= 0 {
		return nil, ErrInvalidParameters
	}
	if cfg.MaxCooldown <= 0 {
		cfg.MaxCooldown = 10 * time.Minute
	}
	if cfg.IdempotencyTtlSec <= 0 {
		cfg.IdempotencyTtlSec = 300
	}

	return &ConvergenceCircuit{
		cfg:     cfg,
		records: make(map[string]*FlapHistory),
		leases:  make(map[string]time.Time),
	}, nil
}

// CheckAndAcquire evaluates whether a reconciliation mutation should proceed.
// Returns an error if the circuit is tripped due to flapping or idempotency duplication.
func (c *ConvergenceCircuit) CheckAndAcquire(resourceKey string, desiredPayload []byte, now time.Time) (string, error) {
	if resourceKey == "" {
		return "", errors.New("resourceKey cannot be empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	hash := sha256Payload(desiredPayload)
	leaseKey := resourceKey + ":" + hash

	// 1. Idempotency Check
	if exp, exists := c.leases[leaseKey]; exists && now.Before(exp) {
		return "", ErrIdempotencyDuplicate
	}

	// 2. Flap Circuit Check
	hist, exists := c.records[resourceKey]
	if !exists {
		hist = &FlapHistory{
			ResourceKey:     resourceKey,
			Transitions:     []time.Time{now},
			TrippedCount:    0,
			State:           CircuitClosed,
			LastAppliedHash: hash,
		}
		c.records[resourceKey] = hist
		c.leases[leaseKey] = now.Add(time.Duration(c.cfg.IdempotencyTtlSec) * time.Second)
		return hash, nil
	}

	// If currently cooling down
	if hist.State == CircuitTripped {
		if now.Before(hist.CooldownUntil) {
			return "", ErrCircuitOpenFlapping
		}
		// Transition to cooling canary probe
		hist.State = CircuitCooling
		hist.Transitions = []time.Time{} // Clear prior flap backlog for probe
	}

	// Clean older transitions outside sliding window
	windowCutoff := now.Add(-time.Duration(c.cfg.FlapWindowSec) * time.Second)
	validTransitions := hist.Transitions[:0]
	for _, t := range hist.Transitions {
		if t.After(windowCutoff) {
			validTransitions = append(validTransitions, t)
		}
	}
	hist.Transitions = validTransitions

	// If payload changed from last applied state, count as a transition
	if hist.LastAppliedHash != hash {
		hist.Transitions = append(hist.Transitions, now)
		hist.LastAppliedHash = hash

		if len(hist.Transitions) >= c.cfg.MaxFlapsPerWindow {
			hist.TrippedCount++
			hist.State = CircuitTripped

			// Exponential backoff cooldown: base * 2^(trip - 1)
			multiplier := 1 << min(hist.TrippedCount-1, 5)
			cooldown := c.cfg.BaseCooldown * time.Duration(multiplier)
			if cooldown > c.cfg.MaxCooldown {
				cooldown = c.cfg.MaxCooldown
			}
			hist.CooldownUntil = now.Add(cooldown)

			return "", ErrCircuitOpenFlapping
		}
	}

	// Otherwise, mutation allowed
	hist.State = CircuitClosed
	c.leases[leaseKey] = now.Add(time.Duration(c.cfg.IdempotencyTtlSec) * time.Second)
	return hash, nil
}

// Reset clears circuit tripping for a resource (manual operator override).
func (c *ConvergenceCircuit) Reset(resourceKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.records, resourceKey)
}

// CleanupEvicted prunes expired leases and dead flap records.
func (c *ConvergenceCircuit) CleanupEvicted(now time.Time) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	evicted := 0
	for k, exp := range c.leases {
		if now.After(exp) {
			delete(c.leases, k)
			evicted++
		}
	}
	return evicted
}

func sha256Payload(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
