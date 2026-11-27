package dispatch

import "testing"

func TestTargetWeightContract(t *testing.T) {
	c := NewTargetWeightContract(1, 100)
	if err := c.Validate(50); err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(150); err == nil {
		t.Fatal("expected error for weight out of range")
	}
}
