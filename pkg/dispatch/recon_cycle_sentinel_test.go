package dispatch

import "testing"

func TestAssertNoDependencyCycle(t *testing.T) {
	if err := AssertNoDependencyCycle("svc-a", []string{"svc-b"}); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := AssertNoDependencyCycle("svc-a", []string{"svc-a"}); err == nil {
		t.Fatal("expected cycle sentinel violation")
	}
}
