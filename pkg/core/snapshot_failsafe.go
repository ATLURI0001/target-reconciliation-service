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
	ErrFailsafeNilSnapshot  = errors.New("failsafe: snapshot is nil")
	ErrSnapshotNotFound     = errors.New("failsafe: snapshot not found")
	ErrDestructionExceeded  = errors.New("failsafe: resource destruction threshold exceeded")
	ErrCorruptResourceState = errors.New("failsafe: corrupt resource record detected")
	ErrReconciliationFailed = errors.New("failsafe: reconciliation transaction failed")
)

// ResourceRecord represents an immutable point-in-time record of a managed infrastructure target.
type ResourceRecord struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	Version    int64             `json:"version"`
	SpecHash   string            `json:"spec_hash"`
	Attributes map[string]string `json:"attributes,omitempty"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// StateSnapshot captures the canonical state tree with cryptographic integrity hashing.
type StateSnapshot struct {
	SnapshotID  string                    `json:"snapshot_id"`
	Epoch       int64                     `json:"epoch"`
	Timestamp   time.Time                 `json:"timestamp"`
	Resources   map[string]ResourceRecord `json:"resources"`
	StateHash   string                    `json:"state_hash"`
	TotalItems  int                       `json:"total_items"`
}

// RollbackRecord logs diagnostic telemetry whenever an automated failsafe rollback is engaged.
type RollbackRecord struct {
	RollbackID        string    `json:"rollback_id"`
	SnapshotID        string    `json:"snapshot_id"`
	TriggerReason     string    `json:"trigger_reason"`
	AttemptedMutations int       `json:"attempted_mutations"`
	Timestamp         time.Time `json:"timestamp"`
	RestoredHash      string    `json:"restored_hash"`
}

// FailsafeTelemetry exports operational metrics for observability scrapers.
type FailsafeTelemetry struct {
	TotalTransactions int   `json:"total_transactions"`
	CommittedCount    int   `json:"committed_count"`
	RollbackCount     int   `json:"rollback_count"`
	ActiveSnapshots   int   `json:"active_snapshots"`
	LastRollbackTime  int64 `json:"last_rollback_time_unix"`
}

// SnapshotFailsafeConfig defines thresholds and limits for transactional reconciliation.
type SnapshotFailsafeConfig struct {
	MaxRollbackHistory      int
	MaxDestructionThreshold int
	AutoRollbackOnPanic     bool
}

// DefaultSnapshotFailsafeConfig returns standard production guard parameters.
func DefaultSnapshotFailsafeConfig() SnapshotFailsafeConfig {
	return SnapshotFailsafeConfig{
		MaxRollbackHistory:      50,
		MaxDestructionThreshold: 5,
		AutoRollbackOnPanic:     true,
	}
}

// SnapshotFailsafeSentinel manages atomic state checkpoints and automated rollbacks on failure.
type SnapshotFailsafeSentinel struct {
	mu                sync.RWMutex
	cfg               SnapshotFailsafeConfig
	snapshots         map[string]*StateSnapshot
	rollbacks         []RollbackRecord
	currentEpoch      int64
	totalTransactions int
	committedCount    int
	rollbackCount     int
	lastRollbackTime  time.Time
}

// NewSnapshotFailsafeSentinel constructs a new thread-safe failsafe manager.
func NewSnapshotFailsafeSentinel(cfg SnapshotFailsafeConfig) *SnapshotFailsafeSentinel {
	if cfg.MaxRollbackHistory <= 0 {
		cfg.MaxRollbackHistory = 50
	}
	if cfg.MaxDestructionThreshold <= 0 {
		cfg.MaxDestructionThreshold = 5
	}
	return &SnapshotFailsafeSentinel{
		cfg:       cfg,
		snapshots: make(map[string]*StateSnapshot),
		rollbacks: make([]RollbackRecord, 0, cfg.MaxRollbackHistory),
	}
}

// ComputeStateHash calculates a deterministic SHA-256 fingerprint over sorted resource keys.
func ComputeStateHash(resources map[string]ResourceRecord) string {
	if len(resources) == 0 {
		return "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // SHA-256 of empty string
	}

	keys := make([]string, 0, len(resources))
	for k := range resources {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		r := resources[k]
		canonical, _ := json.Marshal(struct {
			ID       string            `json:"id"`
			Kind     string            `json:"kind"`
			Version  int64             `json:"version"`
			SpecHash string            `json:"spec_hash"`
			Attrs    map[string]string `json:"attrs,omitempty"`
		}{
			ID:       r.ID,
			Kind:     r.Kind,
			Version:  r.Version,
			SpecHash: r.SpecHash,
			Attrs:    r.Attributes,
		})
		h.Write(canonical)
	}

	return hex.EncodeToString(h.Sum(nil))
}

// CreateSnapshot captures an atomic snapshot of the resource inventory.
func (s *SnapshotFailsafeSentinel) CreateSnapshot(resources map[string]ResourceRecord) (*StateSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentEpoch++
	snapID := fmt.Sprintf("snap-epoch-%d-%d", s.currentEpoch, time.Now().UnixNano())

	// Deep clone resource map
	cloned := make(map[string]ResourceRecord, len(resources))
	for k, v := range resources {
		if v.ID == "" || v.Kind == "" {
			return nil, fmt.Errorf("%w: resource key '%s' missing mandatory fields", ErrCorruptResourceState, k)
		}
		attrs := make(map[string]string, len(v.Attributes))
		for ak, av := range v.Attributes {
			attrs[ak] = av
		}
		cloned[k] = ResourceRecord{
			ID:         v.ID,
			Kind:       v.Kind,
			Version:    v.Version,
			SpecHash:   v.SpecHash,
			Attributes: attrs,
			UpdatedAt:  v.UpdatedAt,
		}
	}

	stateHash := ComputeStateHash(cloned)
	snapshot := &StateSnapshot{
		SnapshotID: snapID,
		Epoch:      s.currentEpoch,
		Timestamp:  time.Now().UTC(),
		Resources:  cloned,
		StateHash:  stateHash,
		TotalItems: len(cloned),
	}

	s.snapshots[snapID] = snapshot
	return snapshot, nil
}

// ExecuteWithFailsafe wraps a reconciliation mutation in an atomic checkpoint and rollback guard.
func (s *SnapshotFailsafeSentinel) ExecuteWithFailsafe(
	initialState map[string]ResourceRecord,
	reconcileFn func(candidate map[string]ResourceRecord) error,
) (map[string]ResourceRecord, *RollbackRecord, error) {
	snapshot, err := s.CreateSnapshot(initialState)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create pre-reconciliation checkpoint: %w", err)
	}

	s.mu.Lock()
	s.totalTransactions++
	s.mu.Unlock()

	// Candidate state clone
	candidate := make(map[string]ResourceRecord, len(initialState))
	for k, v := range initialState {
		candidate[k] = v
	}

	var reconcileErr error
	var panicVal interface{}

	// Execution wrapper with panic recovery
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicVal = r
			}
		}()
		reconcileErr = reconcileFn(candidate)
	}()

	// Evaluate failure conditions
	shouldRollback := false
	var triggerReason string

	if panicVal != nil {
		shouldRollback = true
		triggerReason = fmt.Sprintf("unhandled panic during reconciliation: %v", panicVal)
	} else if reconcileErr != nil {
		shouldRollback = true
		triggerReason = fmt.Sprintf("reconciliation execution error: %v", reconcileErr)
	} else {
		// Invariant checks on candidate state
		deletedCount := 0
		for k := range initialState {
			if _, exists := candidate[k]; !exists {
				deletedCount++
			}
		}

		if deletedCount > s.cfg.MaxDestructionThreshold {
			shouldRollback = true
			triggerReason = fmt.Sprintf("destruction count (%d) exceeded threshold (%d)", deletedCount, s.cfg.MaxDestructionThreshold)
			reconcileErr = ErrDestructionExceeded
		}
	}

	if shouldRollback {
		s.mu.Lock()
		s.rollbackCount++
		s.lastRollbackTime = time.Now().UTC()

		rbID := fmt.Sprintf("rb-%d-%d", s.rollbackCount, time.Now().UnixNano())
		rbRecord := RollbackRecord{
			RollbackID:        rbID,
			SnapshotID:        snapshot.SnapshotID,
			TriggerReason:     triggerReason,
			AttemptedMutations: len(candidate),
			Timestamp:         s.lastRollbackTime,
			RestoredHash:      snapshot.StateHash,
		}

		if len(s.rollbacks) >= s.cfg.MaxRollbackHistory {
			s.rollbacks = s.rollbacks[1:]
		}
		s.rollbacks = append(s.rollbacks, rbRecord)
		s.mu.Unlock()

		// Restore pristine initial state from snapshot
		restored := make(map[string]ResourceRecord, len(snapshot.Resources))
		for k, v := range snapshot.Resources {
			restored[k] = v
		}

		if reconcileErr == nil {
			reconcileErr = fmt.Errorf("%w: %s", ErrReconciliationFailed, triggerReason)
		}
		return restored, &rbRecord, reconcileErr
	}

	s.mu.Lock()
	s.committedCount++
	s.mu.Unlock()

	return candidate, nil, nil
}

// GetTelemetry returns an operational snapshot of transaction and rollback counts.
func (s *SnapshotFailsafeSentinel) GetTelemetry() FailsafeTelemetry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var lastRbUnix int64
	if !s.lastRollbackTime.IsZero() {
		lastRbUnix = s.lastRollbackTime.Unix()
	}

	return FailsafeTelemetry{
		TotalTransactions: s.totalTransactions,
		CommittedCount:    s.committedCount,
		RollbackCount:     s.rollbackCount,
		ActiveSnapshots:   len(s.snapshots),
		LastRollbackTime:  lastRbUnix,
	}
}
