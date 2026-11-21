package core

import (
	"testing"
	"time"
)

func TestConvergenceCircuit_NormalAndIdempotency(t *testing.T) {
	circuit, err := NewConvergenceCircuit(ConvergenceCircuitConfig{
		FlapWindowSec:     60,
		MaxFlapsPerWindow: 3,
		BaseCooldown:      10 * time.Second,
		MaxCooldown:       60 * time.Second,
		IdempotencyTtlSec: 300,
	})
	if err != nil {
		t.Fatalf("failed to init circuit: %v", err)
	}

	t0 := time.Now()
	payloadA := []byte(`{"replicas": 3, "image": "v1.0.0"}`)

	// First acquire: success
	hash1, err := circuit.CheckAndAcquire("deployment/auth-service", payloadA, t0)
	if err != nil || hash1 == "" {
		t.Fatalf("unexpected error on first acquire: %v", err)
	}

	// Immediate duplicate: blocked by idempotency lease
	_, err = circuit.CheckAndAcquire("deployment/auth-service", payloadA, t0.Add(1*time.Second))
	if err != ErrIdempotencyDuplicate {
		t.Fatalf("expected ErrIdempotencyDuplicate, got %v", err)
	}
}

func TestConvergenceCircuit_FlapDampingAndCooldown(t *testing.T) {
	circuit, err := NewConvergenceCircuit(ConvergenceCircuitConfig{
		FlapWindowSec:     60,
		MaxFlapsPerWindow: 3,
		BaseCooldown:      10 * time.Second,
		MaxCooldown:       60 * time.Second,
		IdempotencyTtlSec: 1, // Short lease for testing rapid payload mutations
	})
	if err != nil {
		t.Fatalf("failed to init circuit: %v", err)
	}

	t0 := time.Now()
	payload1 := []byte(`{"state": "A"}`)
	payload2 := []byte(`{"state": "B"}`)
	payload3 := []byte(`{"state": "C"}`)

	// Mutation 1 (state A) -> ok
	_, err = circuit.CheckAndAcquire("ingress/gateway", payload1, t0)
	if err != nil {
		t.Fatalf("mutation 1 failed: %v", err)
	}

	// Mutation 2 (state B at t0 + 2s) -> ok
	_, err = circuit.CheckAndAcquire("ingress/gateway", payload2, t0.Add(2*time.Second))
	if err != nil {
		t.Fatalf("mutation 2 failed: %v", err)
	}

	// Mutation 3 (state C at t0 + 4s) -> reaches 3 flips -> TRIPS CIRCUIT
	_, err = circuit.CheckAndAcquire("ingress/gateway", payload3, t0.Add(4*time.Second))
	if err != ErrCircuitOpenFlapping {
		t.Fatalf("expected ErrCircuitOpenFlapping, got %v", err)
	}

	// Attempt during cooldown (t0 + 8s, cooldown is 10s) -> blocked
	_, err = circuit.CheckAndAcquire("ingress/gateway", payload1, t0.Add(8*time.Second))
	if err != ErrCircuitOpenFlapping {
		t.Fatalf("expected still open flapping at 8s, got %v", err)
	}

	// Attempt after cooldown elapsed (t0 + 15s) -> allowed into cooling recovery
	_, err = circuit.CheckAndAcquire("ingress/gateway", payload1, t0.Add(15*time.Second))
	if err != nil {
		t.Fatalf("expected recovery after cooldown, got %v", err)
	}
}

func TestConvergenceCircuit_ValidationAndReset(t *testing.T) {
	// Invalid parameters
	_, err := NewConvergenceCircuit(ConvergenceCircuitConfig{
		FlapWindowSec:     0,
		MaxFlapsPerWindow: 3,
		BaseCooldown:      10 * time.Second,
	})
	if err != ErrInvalidParameters {
		t.Errorf("expected ErrInvalidParameters for window <= 0, got %v", err)
	}

	circuit, _ := NewConvergenceCircuit(ConvergenceCircuitConfig{
		FlapWindowSec:     60,
		MaxFlapsPerWindow: 3,
		BaseCooldown:      10 * time.Second,
	})

	// Reset overrides tripped state
	circuit.Reset("test-resource")
}
