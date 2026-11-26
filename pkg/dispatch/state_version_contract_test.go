package dispatch

import "testing"

func TestValidateStateTransition(t *testing.T) {
	if err := ValidateStateTransition(4, 5); err != nil {
		t.Fatalf("expected valid transition, got %v", err)
	}
	if err := ValidateStateTransition(4, 6); err == nil {
		t.Fatal("expected version gap contract violation")
	}
}
