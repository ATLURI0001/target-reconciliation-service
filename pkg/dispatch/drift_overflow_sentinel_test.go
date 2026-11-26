package dispatch

import "testing"

func TestCheckDriftLimit(t *testing.T) {
	if err := CheckDriftLimit(5, 10); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := CheckDriftLimit(15, 10); err == nil {
		t.Fatal("expected drift limit sentinel violation")
	}
}
