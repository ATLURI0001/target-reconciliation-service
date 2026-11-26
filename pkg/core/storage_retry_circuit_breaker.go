package core

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"
)

var (
	ErrStorageCircuitOpen   = errors.New("storage_breaker: circuit is OPEN, rejecting transaction")
	ErrStorageConflictStorm = errors.New("storage_breaker: transaction failed after maximum conflict retries")
	ErrStorageDeadlock      = errors.New("storage_breaker: storage deadlock detected")
)

type StorageBreakerState string

const (
	StorageStateClosed   StorageBreakerState = "CLOSED"
	StorageStateOpen     StorageBreakerState = "OPEN"
	StorageStateHalfOpen StorageBreakerState = "HALF_OPEN"
)

// StorageBreakerConfig configures retry policies and circuit breaker limits.
type StorageBreakerConfig struct {
	MaxRetries       int
	InitialBackoff   time.Duration
	MaxBackoff       time.Duration
	FailureThreshold int
	RecoveryTimeout  time.Duration
}

// StorageExecutionResult contains the transaction outcome and degradation status.
type StorageExecutionResult struct {
	Key             string
	Attempts        int
	IsDegraded      bool
	TripReason      string
	CircuitState    StorageBreakerState
	Duration        time.Duration
}

// StorageRetryCircuitBreaker defends state reconciliation stores against optimistic locking storms and deadlocks.
type StorageRetryCircuitBreaker struct {
	mu                  sync.RWMutex
	state               StorageBreakerState
	failureCount        int
	lastStateChangeTime time.Time
	config              StorageBreakerConfig
}

// NewStorageRetryCircuitBreaker constructs a new circuit breaker instance.
func NewStorageRetryCircuitBreaker(cfg StorageBreakerConfig) *StorageRetryCircuitBreaker {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = 10 * time.Millisecond
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 200 * time.Millisecond
	}
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 3
	}
	if cfg.RecoveryTimeout <= 0 {
		cfg.RecoveryTimeout = 200 * time.Millisecond
	}

	return &StorageRetryCircuitBreaker{
		state:               StorageStateClosed,
		lastStateChangeTime: time.Now(),
		config:              cfg,
	}
}

// GetState returns the current state, evaluating time-based transitions.
func (b *StorageRetryCircuitBreaker) GetState(now ...time.Time) StorageBreakerState {
	b.mu.Lock()
	defer b.mu.Unlock()

	curTime := time.Now()
	if len(now) > 0 && !now[0].IsZero() {
		curTime = now[0]
	}

	b.evaluateTransitions(curTime)
	return b.state
}

func (b *StorageRetryCircuitBreaker) evaluateTransitions(now time.Time) {
	if b.state == StorageStateOpen {
		if now.Sub(b.lastStateChangeTime) >= b.config.RecoveryTimeout {
			b.state = StorageStateHalfOpen
			b.lastStateChangeTime = now
		}
	}
}

// ExecuteTransaction executes a storage transaction with exponential backoff and circuit breaking.
func (b *StorageRetryCircuitBreaker) ExecuteTransaction(
	ctx context.Context,
	key string,
	txFn func() error,
	now ...time.Time,
) StorageExecutionResult {
	b.mu.Lock()
	curTime := time.Now()
	if len(now) > 0 && !now[0].IsZero() {
		curTime = now[0]
	}
	b.evaluateTransitions(curTime)

	// 1. Fast reject if circuit is OPEN
	if b.state == StorageStateOpen {
		b.mu.Unlock()
		return StorageExecutionResult{
			Key:          key,
			Attempts:     0,
			IsDegraded:   true,
			TripReason:   "CIRCUIT_OPEN",
			CircuitState: StorageStateOpen,
			Duration:     0,
		}
	}
	b.mu.Unlock()

	startTime := time.Now()
	attempts := 0
	var lastErr error

	for attempts < b.config.MaxRetries {
		attempts++

		// Check context cancellation
		if ctx != nil && ctx.Err() != nil {
			lastErr = ctx.Err()
			break
		}

		err := txFn()
		if err == nil {
			// Success!
			b.onSuccess(curTime)
			return StorageExecutionResult{
				Key:          key,
				Attempts:     attempts,
				IsDegraded:   false,
				CircuitState: b.GetState(curTime),
				Duration:     time.Since(startTime),
			}
		}

		lastErr = err

		// Calculate backoff: InitialBackoff * 2^(attempts-1) capped at MaxBackoff
		backoff := float64(b.config.InitialBackoff) * math.Pow(2, float64(attempts-1))
		sleepDur := time.Duration(math.Min(backoff, float64(b.config.MaxBackoff)))

		if attempts < b.config.MaxRetries {
			select {
			case <-time.After(sleepDur):
			case <-ctx.Done():
				lastErr = ctx.Err()
				break
			}
		}
	}

	// Transaction failed across all attempts
	b.onFailure(curTime)

	tripReason := "OCC_CONFLICT"
	if errors.Is(lastErr, ErrStorageDeadlock) {
		tripReason = "DEADLOCK"
	}

	return StorageExecutionResult{
		Key:          key,
		Attempts:     attempts,
		IsDegraded:   true,
		TripReason:   tripReason,
		CircuitState: b.GetState(curTime),
		Duration:     time.Since(startTime),
	}
}

func (b *StorageRetryCircuitBreaker) onSuccess(now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == StorageStateHalfOpen {
		b.state = StorageStateClosed
		b.failureCount = 0
		b.lastStateChangeTime = now
	} else {
		if b.failureCount > 0 {
			b.failureCount--
		}
	}
}

func (b *StorageRetryCircuitBreaker) onFailure(now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failureCount++
	if b.failureCount >= b.config.FailureThreshold || b.state == StorageStateHalfOpen {
		b.state = StorageStateOpen
		b.lastStateChangeTime = now
	}
}

// Reset clears failure counts and resets breaker to CLOSED.
func (b *StorageRetryCircuitBreaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = StorageStateClosed
	b.failureCount = 0
	b.lastStateChangeTime = time.Now()
}
