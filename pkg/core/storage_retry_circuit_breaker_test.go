package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStorageRetryCircuitBreaker_NormalOperation(t *testing.T) {
	cfg := StorageBreakerConfig{
		MaxRetries:       3,
		InitialBackoff:   5 * time.Millisecond,
		FailureThreshold: 2,
		RecoveryTimeout:  100 * time.Millisecond,
	}
	breaker := NewStorageRetryCircuitBreaker(cfg)

	// Successful on attempt 1
	res := breaker.ExecuteTransaction(context.Background(), "target_k8s_cluster", func() error {
		return nil
	})

	if res.IsDegraded {
		t.Fatal("expected non-degraded execution")
	}
	if res.Attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", res.Attempts)
	}
	if res.CircuitState != StorageStateClosed {
		t.Fatalf("expected CLOSED state, got %s", res.CircuitState)
	}
}

func TestStorageRetryCircuitBreaker_RetryAndCircuitTrip(t *testing.T) {
	cfg := StorageBreakerConfig{
		MaxRetries:       2,
		InitialBackoff:   5 * time.Millisecond,
		FailureThreshold: 2,
		RecoveryTimeout:  100 * time.Millisecond,
	}
	breaker := NewStorageRetryCircuitBreaker(cfg)

	t0 := time.Now()

	// Failure 1
	r1 := breaker.ExecuteTransaction(context.Background(), "key1", func() error {
		return errors.New("optimistic lock failure")
	}, t0)
	if !r1.IsDegraded || r1.Attempts != 2 {
		t.Fatalf("expected degraded after 2 retries, got %+v", r1)
	}
	if breaker.GetState(t0) != StorageStateClosed {
		t.Fatalf("expected state CLOSED after 1 failure storm, got %s", breaker.GetState(t0))
	}

	// Failure 2 (reaches failureThreshold of 2) -> trips to OPEN
	r2 := breaker.ExecuteTransaction(context.Background(), "key2", func() error {
		return errors.New("optimistic lock failure")
	}, t0)
	if !r2.IsDegraded {
		t.Fatal("expected degraded execution")
	}
	if breaker.GetState(t0) != StorageStateOpen {
		t.Fatalf("expected state OPEN, got %s", breaker.GetState(t0))
	}

	// Fast-reject during OPEN state
	r3 := breaker.ExecuteTransaction(context.Background(), "key3", func() error {
		return nil
	}, t0)
	if !r3.IsDegraded || r3.TripReason != "CIRCUIT_OPEN" {
		t.Fatalf("expected fast rejection, got %+v", r3)
	}
	if r3.Attempts != 0 {
		t.Fatalf("expected 0 attempts during open rejection, got %d", r3.Attempts)
	}
}

func TestStorageRetryCircuitBreaker_RecoveryLifecycle(t *testing.T) {
	cfg := StorageBreakerConfig{
		MaxRetries:       2,
		InitialBackoff:   5 * time.Millisecond,
		FailureThreshold: 2,
		RecoveryTimeout:  100 * time.Millisecond,
	}
	breaker := NewStorageRetryCircuitBreaker(cfg)

	t0 := time.Now()
	// Force breaker to OPEN
	breaker.ExecuteTransaction(context.Background(), "k1", func() error { return errors.New("err"); }, t0)
	breaker.ExecuteTransaction(context.Background(), "k2", func() error { return errors.New("err"); }, t0)
	if breaker.GetState(t0) != StorageStateOpen {
		t.Fatalf("expected OPEN state")
	}

	// After recovery timeout (150ms > 100ms)
	tHalf := t0.Add(150 * time.Millisecond)
	if breaker.GetState(tHalf) != StorageStateHalfOpen {
		t.Fatalf("expected HALF_OPEN state, got %s", breaker.GetState(tHalf))
	}

	// Successful probe closes circuit
	rProbe := breaker.ExecuteTransaction(context.Background(), "k_probe", func() error {
		return nil
	}, tHalf)

	if rProbe.IsDegraded {
		t.Fatal("expected probe to succeed")
	}
	if breaker.GetState(tHalf) != StorageStateClosed {
		t.Fatalf("expected state CLOSED after probe, got %s", breaker.GetState(tHalf))
	}
}
