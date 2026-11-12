// Package store provides transactional repositories for the initial single-tenant lab.
package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/ATLURI0001/target-reconciliation-service/internal/core"
)

var ErrAuditMutation = errors.New("committed audit history cannot be changed")

// clone prevents mutable map/slice references escaping transaction boundaries.
func clone(state core.State) (core.State, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return core.State{}, fmt.Errorf("encode state: %w", err)
	}
	var result core.State
	if err = json.Unmarshal(data, &result); err != nil {
		return core.State{}, fmt.Errorf("decode state: %w", err)
	}
	return result, nil
}

func validateAudit(previous, next []core.Audit) error {
	if len(next) < len(previous) {
		return ErrAuditMutation
	}
	// JSON comparison treats timestamp locations consistently after persistence.
	a, err := json.Marshal(previous)
	if err != nil {
		return err
	}
	b, err := json.Marshal(next[:len(previous)])
	if err != nil {
		return err
	}
	if !bytes.Equal(a, b) {
		return ErrAuditMutation
	}
	for i := len(previous); i < len(next); i++ {
		if next[i].Sequence != i+1 || next[i].At.IsZero() || next[i].Actor == "" || next[i].Action == "" {
			return fmt.Errorf("invalid audit entry at sequence %d", i+1)
		}
	}
	return nil
}

// Memory is for isolated tests only. The executable requires PostgreSQL.
// Update callbacks must not call back into the same repository.
type Memory struct {
	mu    sync.Mutex
	state core.State
}

func NewMemory() *Memory { return &Memory{state: core.NewState()} }
func (m *Memory) View(ctx context.Context) (core.State, error) {
	if err := ctx.Err(); err != nil {
		return core.State{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return core.State{}, err
	}
	return clone(m.state)
}
func (m *Memory) Update(ctx context.Context, fn func(*core.State) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	next, err := clone(m.state)
	if err != nil {
		return err
	}
	if err = fn(&next); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = validateAudit(m.state.Audit, next.Audit); err != nil {
		return err
	}
	// Callback references must not alias committed state either.
	committed, err := clone(next)
	if err != nil {
		return err
	}
	m.state = committed
	return nil
}

var _ core.Store = (*Memory)(nil)
