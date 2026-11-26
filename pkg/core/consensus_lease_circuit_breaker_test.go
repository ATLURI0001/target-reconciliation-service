package core

import (
	"errors"
	"testing"
	"time"
)

func TestConsensusLeaseCircuitBreaker_NormalOperation(t *testing.T) {
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	cfg := DefaultConsensusLeaseCircuitBreakerConfig()
	cfg.NowFunc = func() time.Time { return now }

	cb := NewConsensusLeaseCircuitBreaker(cfg)

	local := []ReconciliationTarget{
		{ID: "target-1", State: "CACHED"},
	}

	remote := []ReconciliationTarget{
		{ID: "target-1", State: "ACTIVE"},
		{ID: "target-2", State: "ACTIVE"},
	}

	res := cb.ExecuteWithFallback(local, func() ([]ReconciliationTarget, error) {
		return remote, nil
	})

	if res.IsDegraded {
		t.Fatalf("expected healthy sync, got degraded")
	}
	if len(res.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(res.Targets))
	}
	if res.State != StateClosed {
		t.Fatalf("expected StateClosed, got %v", res.State)
	}
}

func TestConsensusLeaseCircuitBreaker_FallbackOnTimeoutAndTripping(t *testing.T) {
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	cfg := DefaultConsensusLeaseCircuitBreakerConfig()
	cfg.FailureThreshold = 2
	cfg.MaxLeaseTimeout = 10 * time.Millisecond
	cfg.RecoveryTimeout = 200 * time.Millisecond
	cfg.NowFunc = func() time.Time { return now }

	cb := NewConsensusLeaseCircuitBreaker(cfg)

	local := []ReconciliationTarget{{ID: "cached-fallback", State: "DEGRADED"}}

	// Failure 1: connection error
	res1 := cb.ExecuteWithFallback(local, func() ([]ReconciliationTarget, error) {
		return nil, errors.New("network partition")
	})
	if !res1.IsDegraded || res1.FallbackMode != "LOCAL_READONLY_SNAPSHOT" {
		t.Fatalf("expected local fallback on error, got %v", res1)
	}
	if cb.GetState(now) != StateClosed {
		t.Fatalf("circuit should remain closed after 1 failure, got %v", cb.GetState(now))
	}

	// Failure 2: reaches threshold -> trips to OPEN
	res2 := cb.ExecuteWithFallback(local, func() ([]ReconciliationTarget, error) {
		return nil, errors.New("network partition 2")
	})
	if !res2.IsDegraded {
		t.Fatalf("expected fallback")
	}
	if cb.GetState(now) != StateOpen {
		t.Fatalf("circuit should be OPEN after 2 failures, got %v", cb.GetState(now))
	}

	// Third call should short-circuit immediately without calling remote function
	called := false
	res3 := cb.ExecuteWithFallback(local, func() ([]ReconciliationTarget, error) {
		called = true
		return nil, nil
	})
	if called {
		t.Fatalf("remote sync should not be called when circuit is OPEN")
	}
	if res3.State != StateOpen || !res3.IsDegraded {
		t.Fatalf("expected fast short-circuit")
	}

	// Advance time past recovery timeout
	now = now.Add(250 * time.Millisecond)
	if cb.GetState(now) != StateHalfOpen {
		t.Fatalf("circuit should transition to HALF_OPEN, got %v", cb.GetState(now))
	}

	// Successful probe in HALF_OPEN restores StateClosed
	res4 := cb.ExecuteWithFallback(local, func() ([]ReconciliationTarget, error) {
		return []ReconciliationTarget{{ID: "recovered"}}, nil
	})
	if res4.IsDegraded {
		t.Fatalf("expected successful recovery")
	}
	if cb.GetState(now) != StateClosed {
		t.Fatalf("circuit should return to StateClosed, got %v", cb.GetState(now))
	}
}
