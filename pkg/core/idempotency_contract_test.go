package core

import (
	"errors"
	"testing"
	"time"
)

func TestIdempotencyContract_EnforceExactlyOnce(t *testing.T) {
	contract := NewIdempotencyContract(1 * time.Minute)
	now := time.Now()

	// 1. Acquire new key
	completed, err := contract.Acquire("req-123", "hash-abc", now)
	if err != nil || completed {
		t.Fatalf("expected fresh acquire, got completed=%v, err=%v", completed, err)
	}

	// 2. Concurrent in-flight check
	_, err = contract.Acquire("req-123", "hash-abc", now)
	if !errors.Is(err, ErrIdempotencyInFlight) {
		t.Fatalf("expected ErrIdempotencyInFlight, got %v", err)
	}

	// 3. Complete key
	contract.Complete("req-123")

	// 4. Replay with same hash -> returns completed=true, nil
	completed, err = contract.Acquire("req-123", "hash-abc", now)
	if err != nil || !completed {
		t.Fatalf("expected completed=true on replay, got completed=%v, err=%v", completed, err)
	}

	// 5. Reuse with conflicting payload hash -> error
	_, err = contract.Acquire("req-123", "hash-different", now)
	if !errors.Is(err, ErrIdempotencyHashMismatch) {
		t.Fatalf("expected ErrIdempotencyHashMismatch, got %v", err)
	}
}
