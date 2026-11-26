package core

import "testing"

func TestDriftTelemetryRotary_Summary(t *testing.T) {
	rotary := NewDriftTelemetryRotary(5)
	for i := 1; i <= 7; i++ {
		rotary.Record("cluster-us", 2, int64(i*15))
	}

	summary := rotary.GetSummary()
	if summary.SampleCount != 5 || summary.TotalDrifted != 10 || summary.P50ScanMillis <= 0 {
		t.Fatalf("unexpected drift summary: %+v", summary)
	}
}
