package core

import (
	"errors"
	"sync"
	"time"
)

var (
	// ErrLeaseTimeout indicates distributed lease acquisition or renewal timed out.
	ErrLeaseTimeout = errors.New("consensus_breaker: distributed consensus lease acquisition timed out")
	// ErrConnectionRefused indicates network partition or cluster unreachable.
	ErrConnectionRefused = errors.New("consensus_breaker: connection refused by consensus cluster")
	// ErrCircuitBreakerOpen indicates requests are being fast-failed to local fallback.
	ErrCircuitBreakerOpen = errors.New("consensus_breaker: circuit is OPEN; consensus cluster partitioned")
)

// ConsensusBreakerState represents the current circuit breaker operational mode.
type ConsensusBreakerState string

const (
	StateClosed   ConsensusBreakerState = "CLOSED"
	StateOpen     ConsensusBreakerState = "OPEN"
	StateHalfOpen ConsensusBreakerState = "HALF_OPEN"
)

// ConsensusLeaseCircuitBreakerConfig defines configuration parameters.
type ConsensusLeaseCircuitBreakerConfig struct {
	FailureThreshold int
	RecoveryTimeout  time.Duration
	MaxLeaseTimeout  time.Duration
	NowFunc          func() time.Time
}

// DefaultConsensusLeaseCircuitBreakerConfig provides production-ready defaults.
func DefaultConsensusLeaseCircuitBreakerConfig() ConsensusLeaseCircuitBreakerConfig {
	return ConsensusLeaseCircuitBreakerConfig{
		FailureThreshold: 3,
		RecoveryTimeout:  2 * time.Second,
		MaxLeaseTimeout:  100 * time.Millisecond,
		NowFunc:          time.Now,
	}
}

// ReconciliationTarget represents a target resource in the cluster.
type ReconciliationTarget struct {
	ID        string
	SpecHash  string
	State     string
	Version   int64
	UpdatedAt time.Time
}

// ConsensusExecutionResult details outcome of a lease renewal or sync operation.
type ConsensusExecutionResult struct {
	Targets      []ReconciliationTarget
	IsDegraded   bool
	State        ConsensusBreakerState
	FallbackMode string
	ErrorReason  string
	Duration     time.Duration
}

// ConsensusLeaseCircuitBreaker protects reconciliation workers against leader lease partitions.
type ConsensusLeaseCircuitBreaker struct {
	mu                  sync.RWMutex
	cfg                 ConsensusLeaseCircuitBreakerConfig
	state               ConsensusBreakerState
	failureCount        int
	lastStateChangeTime time.Time
}

// NewConsensusLeaseCircuitBreaker constructs a new circuit breaker instance.
func NewConsensusLeaseCircuitBreaker(cfg ConsensusLeaseCircuitBreakerConfig) *ConsensusLeaseCircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 3
	}
	if cfg.RecoveryTimeout <= 0 {
		cfg.RecoveryTimeout = 2 * time.Second
	}
	if cfg.MaxLeaseTimeout <= 0 {
		cfg.MaxLeaseTimeout = 100 * time.Millisecond
	}
	if cfg.NowFunc == nil {
		cfg.NowFunc = time.Now
	}

	return &ConsensusLeaseCircuitBreaker{
		cfg:                 cfg,
		state:               StateClosed,
		lastStateChangeTime: cfg.NowFunc(),
	}
}

// GetState returns the current circuit state, handling automatic transition to HALF_OPEN.
func (cb *ConsensusLeaseCircuitBreaker) GetState(now time.Time) ConsensusBreakerState {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateOpen {
		if now.Sub(cb.lastStateChangeTime) >= cb.cfg.RecoveryTimeout {
			cb.state = StateHalfOpen
			cb.lastStateChangeTime = now
		}
	}
	return cb.state
}

// Reset restores the circuit breaker to clean CLOSED state.
func (cb *ConsensusLeaseCircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failureCount = 0
	cb.lastStateChangeTime = cb.cfg.NowFunc()
}

// ExecuteWithFallback executes consensus sync with timeout protection and local snapshot fallback.
func (cb *ConsensusLeaseCircuitBreaker) ExecuteWithFallback(
	localSnapshot []ReconciliationTarget,
	remoteSyncFn func() ([]ReconciliationTarget, error),
) ConsensusExecutionResult {
	now := cb.cfg.NowFunc()
	currentState := cb.GetState(now)

	// Fast short-circuit if OPEN
	if currentState == StateOpen {
		return ConsensusExecutionResult{
			Targets:      localSnapshot,
			IsDegraded:   true,
			State:        StateOpen,
			FallbackMode: "LOCAL_READONLY_SNAPSHOT",
			ErrorReason:  ErrCircuitBreakerOpen.Error(),
			Duration:     0,
		}
	}

	start := cb.cfg.NowFunc()

	type syncRes struct {
		targets []ReconciliationTarget
		err     error
	}

	resCh := make(chan syncRes, 1)
	go func() {
		t, err := remoteSyncFn()
		resCh <- syncRes{targets: t, err: err}
	}()

	select {
	case res := <-resCh:
		dur := cb.cfg.NowFunc().Sub(start)
		if res.err != nil {
			cb.recordFailure(now)
			return ConsensusExecutionResult{
				Targets:      localSnapshot,
				IsDegraded:   true,
				State:        cb.state,
				FallbackMode: "LOCAL_READONLY_SNAPSHOT",
				ErrorReason:  res.err.Error(),
				Duration:     dur,
			}
		}

		cb.recordSuccess()
		return ConsensusExecutionResult{
			Targets:    res.targets,
			IsDegraded: false,
			State:      cb.state,
			Duration:   dur,
		}

	case <-time.After(cb.cfg.MaxLeaseTimeout):
		dur := cb.cfg.NowFunc().Sub(start)
		cb.recordFailure(now)
		return ConsensusExecutionResult{
			Targets:      localSnapshot,
			IsDegraded:   true,
			State:        cb.state,
			FallbackMode: "LOCAL_READONLY_SNAPSHOT",
			ErrorReason:  ErrLeaseTimeout.Error(),
			Duration:     dur,
		}
	}
}

func (cb *ConsensusLeaseCircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateHalfOpen {
		cb.state = StateClosed
		cb.failureCount = 0
		cb.lastStateChangeTime = cb.cfg.NowFunc()
	} else if cb.failureCount > 0 {
		cb.failureCount--
	}
}

func (cb *ConsensusLeaseCircuitBreaker) recordFailure(now time.Time) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	if cb.state == StateHalfOpen || cb.failureCount >= cb.cfg.FailureThreshold {
		cb.state = StateOpen
		cb.lastStateChangeTime = now
	}
}
