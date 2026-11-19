package store

import (
	"sync"
	"time"
)

// AuditEvent records administrative state modifications.
type AuditEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
}

// RingAuditTrail maintains a fixed-capacity in-memory audit log.
type RingAuditTrail struct {
	mu       sync.RWMutex
	capacity int
	events   []AuditEvent
}

// NewRingAuditTrail creates a RingAuditTrail with maximum event retention.
func NewRingAuditTrail(capacity int) *RingAuditTrail {
	if capacity <= 0 {
		capacity = 100
	}
	return &RingAuditTrail{
		capacity: capacity,
		events:   make([]AuditEvent, 0, capacity),
	}
}

// Record appends an audit entry, evicting the oldest entry when capacity is exceeded.
func (r *RingAuditTrail) Record(event AuditEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.events) >= r.capacity {
		r.events = r.events[1:]
	}
	r.events = append(r.events, event)
}

// Count returns the number of currently retained audit events.
func (r *RingAuditTrail) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.events)
}
