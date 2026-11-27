package dispatch

import "testing"

func TestValidateTargetHealth(t *testing.T) {
	if err := ValidateTargetHealth("healthy"); err != nil {
		t.Fatalf("expected valid health, got %v", err)
	}
	if err := ValidateTargetHealth("unknown"); err == nil {
		t.Fatal("expected health contract violation")
	}
}
