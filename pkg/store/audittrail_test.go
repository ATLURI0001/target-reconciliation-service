package store

import (
	"sync"
	"testing"
	"time"
)

func TestRingAuditTrail(t *testing.T) {
	trail := NewRingAuditTrail(3)

	trail.Record(AuditEvent{Actor: "alice", Action: "CREATE", Resource: "res-1", Timestamp: time.Now()})
	trail.Record(AuditEvent{Actor: "bob", Action: "UPDATE", Resource: "res-1", Timestamp: time.Now()})
	if trail.Count() != 2 {
		t.Fatalf("expected count 2, got %d", trail.Count())
	}

	trail.Record(AuditEvent{Actor: "charlie", Action: "DELETE", Resource: "res-2", Timestamp: time.Now()})
	if trail.Count() != 3 {
		t.Fatalf("expected count 3, got %d", trail.Count())
	}

	// 4th event causes ring buffer to evict oldest (capacity 3)
	trail.Record(AuditEvent{Actor: "dave", Action: "CREATE", Resource: "res-3", Timestamp: time.Now()})
	if trail.Count() != 3 {
		t.Fatalf("expected count to stay at capacity 3, got %d", trail.Count())
	}
}

func TestRingAuditTrail_Concurrency(t *testing.T) {
	trail := NewRingAuditTrail(50)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			trail.Record(AuditEvent{
				Actor:     "worker",
				Action:    "EXEC",
				Resource:  "res",
				Timestamp: time.Now(),
			})
			_ = trail.Count()
		}(i)
	}

	wg.Wait()
	if trail.Count() != 50 {
		t.Fatalf("expected capped count 50 under concurrent writes, got %d", trail.Count())
	}
}
