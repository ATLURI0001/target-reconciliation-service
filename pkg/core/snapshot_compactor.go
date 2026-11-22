package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var (
	ErrNonMonotonicSequence      = errors.New("delta sequence must be strictly monotonically increasing")
	ErrInvalidDelta              = errors.New("state delta contains invalid fields or empty resource id")
	ErrSnapshotIntegrityViolation = errors.New("snapshot integrity verification failed")
	ErrNilSnapshot               = errors.New("snapshot cannot be nil")
)

// DeltaOp defines the type of state mutation event.
type DeltaOp string

const (
	DeltaCreate DeltaOp = "CREATE"
	DeltaUpdate DeltaOp = "UPDATE"
	DeltaPatch  DeltaOp = "PATCH"
	DeltaDelete DeltaOp = "DELETE"
)

// StateDelta represents an incremental state mutation event.
type StateDelta struct {
	Sequence   uint64         `json:"sequence"`
	ResourceID string         `json:"resource_id"`
	Op         DeltaOp        `json:"op"`
	Payload    map[string]any `json:"payload,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
	Checksum   string         `json:"checksum"`
}

// ComputeChecksum generates a deterministic SHA-256 signature for the delta.
func (d *StateDelta) ComputeChecksum() string {
	payloadJSON, _ := json.Marshal(d.Payload)
	raw := fmt.Sprintf("%d:%s:%s:%s:%d", d.Sequence, d.ResourceID, d.Op, string(payloadJSON), d.Timestamp.UnixNano())
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// Snapshot represents a point-in-time consolidated view of all managed resources.
type Snapshot struct {
	SnapshotID    string                    `json:"snapshot_id"`
	BaseSequence  uint64                    `json:"base_sequence"`
	CreatedAt     time.Time                 `json:"created_at"`
	Resources     map[string]map[string]any `json:"resources"`
	ResourceCount int                       `json:"resource_count"`
	SnapshotHash  string                    `json:"snapshot_hash"`
}

// CompactorConfig configures compaction behavior.
type CompactorConfig struct {
	CompactionThreshold int `json:"compaction_threshold"`
	RetentionSnapshots  int `json:"retention_snapshots"`
}

// DefaultCompactorConfig provides standard operational limits.
func DefaultCompactorConfig() CompactorConfig {
	return CompactorConfig{
		CompactionThreshold: 50,
		RetentionSnapshots:  5,
	}
}

// SnapshotCompactor coordinates log compaction and incremental delta replay.
type SnapshotCompactor struct {
	mu            sync.RWMutex
	config        CompactorConfig
	latestSnapshot *Snapshot
	snapshots     []*Snapshot
	deltas        []StateDelta
	lastSequence  uint64
}

// NewSnapshotCompactor initializes a compactor instance.
func NewSnapshotCompactor(cfg CompactorConfig) *SnapshotCompactor {
	if cfg.CompactionThreshold <= 0 {
		cfg.CompactionThreshold = 50
	}
	if cfg.RetentionSnapshots <= 0 {
		cfg.RetentionSnapshots = 5
	}
	return &SnapshotCompactor{
		config:    cfg,
		snapshots: make([]*Snapshot, 0),
		deltas:    make([]StateDelta, 0),
	}
}

// AppendDelta records and validates an incoming mutation delta.
// If delta.Sequence == 0, the next monotonic sequence number is automatically assigned.
func (sc *SnapshotCompactor) AppendDelta(delta StateDelta) error {
	if delta.ResourceID == "" || delta.Op == "" {
		return ErrInvalidDelta
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	expectedSeq := sc.lastSequence + 1
	if delta.Sequence == 0 {
		delta.Sequence = expectedSeq
	} else if delta.Sequence != expectedSeq {
		return fmt.Errorf("%w: expected %d, got %d", ErrNonMonotonicSequence, expectedSeq, delta.Sequence)
	}

	if delta.Timestamp.IsZero() {
		delta.Timestamp = time.Now().UTC()
	}

	if delta.Checksum == "" {
		delta.Checksum = delta.ComputeChecksum()
	}

	sc.deltas = append(sc.deltas, delta)
	sc.lastSequence = delta.Sequence
	return nil
}

// ShouldCompact returns true if uncompacted delta count meets or exceeds threshold.
func (sc *SnapshotCompactor) ShouldCompact() bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return len(sc.deltas) >= sc.config.CompactionThreshold
}

// Compact consolidates in-memory deltas into a new snapshot and purges compacted history.
func (sc *SnapshotCompactor) Compact() (*Snapshot, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if len(sc.deltas) == 0 {
		if sc.latestSnapshot != nil {
			return sc.latestSnapshot, nil
		}
		// Create empty initial snapshot
		snap := &Snapshot{
			SnapshotID:    fmt.Sprintf("snap-%d", time.Now().UnixNano()),
			BaseSequence:  0,
			CreatedAt:     time.Now().UTC(),
			Resources:     make(map[string]map[string]any),
			ResourceCount: 0,
		}
		snap.SnapshotHash = sc.computeResourcesHash(snap.Resources)
		sc.latestSnapshot = snap
		sc.snapshots = append(sc.snapshots, snap)
		return snap, nil
	}

	// Clone state from latest snapshot if available
	state := make(map[string]map[string]any)
	if sc.latestSnapshot != nil && sc.latestSnapshot.Resources != nil {
		for k, v := range sc.latestSnapshot.Resources {
			copyMap := make(map[string]any)
			for propK, propV := range v {
				copyMap[propK] = propV
			}
			state[k] = copyMap
		}
	}

	var maxSeq uint64
	for _, delta := range sc.deltas {
		if delta.Sequence > maxSeq {
			maxSeq = delta.Sequence
		}

		switch delta.Op {
		case DeltaCreate, DeltaUpdate:
			target := make(map[string]any)
			if delta.Payload != nil {
				for pk, pv := range delta.Payload {
					target[pk] = pv
				}
			}
			state[delta.ResourceID] = target

		case DeltaPatch:
			existing, exists := state[delta.ResourceID]
			if !exists {
				existing = make(map[string]any)
			}
			if delta.Payload != nil {
				for pk, pv := range delta.Payload {
					existing[pk] = pv
				}
			}
			state[delta.ResourceID] = existing

		case DeltaDelete:
			delete(state, delta.ResourceID)
		}
	}

	snap := &Snapshot{
		SnapshotID:    fmt.Sprintf("snap-%d-%d", maxSeq, time.Now().UnixNano()),
		BaseSequence:  maxSeq,
		CreatedAt:     time.Now().UTC(),
		Resources:     state,
		ResourceCount: len(state),
	}
	snap.SnapshotHash = sc.computeResourcesHash(state)

	sc.latestSnapshot = snap
	sc.snapshots = append(sc.snapshots, snap)

	// Enforce snapshot retention limit
	if len(sc.snapshots) > sc.config.RetentionSnapshots {
		sc.snapshots = sc.snapshots[len(sc.snapshots)-sc.config.RetentionSnapshots:]
	}

	// Purge compacted deltas
	sc.deltas = make([]StateDelta, 0)

	return snap, nil
}

// computeResourcesHash computes a deterministic SHA-256 hash over canonical sorted keys.
func (sc *SnapshotCompactor) computeResourcesHash(resources map[string]map[string]any) string {
	keys := make([]string, 0, len(resources))
	for k := range resources {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		subProps := resources[k]
		subKeys := make([]string, 0, len(subProps))
		for sp := range subProps {
			subKeys = append(subKeys, sp)
		}
		sort.Strings(subKeys)

		h.Write([]byte(fmt.Sprintf("[%s]", k)))
		for _, sp := range subKeys {
			valJSON, _ := json.Marshal(subProps[sp])
			h.Write([]byte(fmt.Sprintf("%s=%s;", sp, string(valJSON))))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// GetLatestSnapshot returns the most recently compacted snapshot.
func (sc *SnapshotCompactor) GetLatestSnapshot() *Snapshot {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.latestSnapshot
}

// PendingDeltasCount returns the number of deltas waiting to be compacted.
func (sc *SnapshotCompactor) PendingDeltasCount() int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return len(sc.deltas)
}

// VerifySnapshotHash validates that a snapshot's hash matches its canonical content.
func (sc *SnapshotCompactor) VerifySnapshotHash(snap *Snapshot) bool {
	if snap == nil {
		return false
	}
	expected := sc.computeResourcesHash(snap.Resources)
	return expected == snap.SnapshotHash
}

// ReplayFromSnapshot applies a set of deltas on top of a snapshot to construct target state.
func (sc *SnapshotCompactor) ReplayFromSnapshot(snap *Snapshot, deltas []StateDelta) (map[string]map[string]any, error) {
	if snap == nil {
		return nil, ErrNilSnapshot
	}

	if !sc.VerifySnapshotHash(snap) {
		return nil, ErrSnapshotIntegrityViolation
	}

	// Clone snapshot state
	result := make(map[string]map[string]any)
	for k, v := range snap.Resources {
		copyMap := make(map[string]any)
		for pk, pv := range v {
			copyMap[pk] = pv
		}
		result[k] = copyMap
	}

	for _, d := range deltas {
		if d.Sequence <= snap.BaseSequence {
			continue // skip already compacted deltas
		}

		switch d.Op {
		case DeltaCreate, DeltaUpdate:
			target := make(map[string]any)
			for pk, pv := range d.Payload {
				target[pk] = pv
			}
			result[d.ResourceID] = target
		case DeltaPatch:
			existing, exists := result[d.ResourceID]
			if !exists {
				existing = make(map[string]any)
			}
			for pk, pv := range d.Payload {
				existing[pk] = pv
			}
			result[d.ResourceID] = existing
		case DeltaDelete:
			delete(result, d.ResourceID)
		}
	}

	return result, nil
}
