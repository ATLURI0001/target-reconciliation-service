package dispatch

import (
	"testing"
	"time"
)

func TestRequeueProfiler(t *testing.T) {
	p := &RequeueProfiler{}
	p.Record(time.Now())
	if p.Len() != 1 {
		t.Fatalf("expected 1 sample, got %d", p.Len())
	}
}
