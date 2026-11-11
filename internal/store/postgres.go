package store

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ATLURI0001/resilienceplane/internal/core"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/001_initial.sql
var schema string

type Postgres struct{ pool *pgxpool.Pool }

// NewPostgres applies the initial schema under a transaction-scoped advisory
// lock. The initial lab uses one identity for bootstrap and runtime; production
// must split migration ownership from narrowly granted runtime credentials.
func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid database connection configuration")
	}
	config.MaxConns = 8
	config.MinConns = 0
	config.MaxConnIdleTime = time.Minute
	config.ConnConfig.ConnectTimeout = 5 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	p := &Postgres{pool: pool}
	if err = p.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return p, nil
}
func (p *Postgres) migrate(ctx context.Context) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(1380994097)"); err != nil {
		return fmt.Errorf("migration lock: %w", err)
	}
	if _, err = tx.Exec(ctx, schema); err != nil {
		return fmt.Errorf("schema migration: %w", err)
	}
	initial, err := json.Marshal(core.NewState())
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO rp_state(singleton,document) VALUES(true,$1) ON CONFLICT DO NOTHING", initial); err != nil {
		return fmt.Errorf("initialize state: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}
func (p *Postgres) Close() { p.pool.Close() }
func (p *Postgres) View(ctx context.Context) (core.State, error) {
	var data []byte
	if err := p.pool.QueryRow(ctx, "SELECT document FROM rp_state WHERE singleton=true").Scan(&data); err != nil {
		return core.State{}, fmt.Errorf("read state: %w", err)
	}
	var state core.State
	if err := json.Unmarshal(data, &state); err != nil {
		return core.State{}, fmt.Errorf("decode persisted state: %w", err)
	}
	return state, nil
}

// Update serializes all mutations using the aggregate row lock. This is a
// correctness-first prototype with bounded scale, not a sharded event store.
// Callbacks contain only in-memory decisions; external I/O must happen after commit.
func (p *Postgres) Update(ctx context.Context, fn func(*core.State) error) error {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin update: %w", err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SET LOCAL lock_timeout = '5s'"); err != nil {
		return err
	}
	var data []byte
	if err = tx.QueryRow(ctx, "SELECT document FROM rp_state WHERE singleton=true FOR UPDATE").Scan(&data); err != nil {
		return fmt.Errorf("lock state: %w", err)
	}
	var state core.State
	if err = json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("decode persisted state: %w", err)
	}
	previous, err := clone(state)
	if err != nil {
		return err
	}
	if err = fn(&state); err != nil {
		return err
	}
	if err = validateAudit(previous.Audit, state.Audit); err != nil {
		return err
	}
	data, err = json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	for _, entry := range state.Audit[len(previous.Audit):] {
		if _, err = tx.Exec(ctx, "INSERT INTO rp_audit(sequence,at,actor,action,resource,result) VALUES($1,$2,$3,$4,$5,$6)", entry.Sequence, entry.At, entry.Actor, entry.Action, entry.Resource, entry.Result); err != nil {
			return fmt.Errorf("append audit: %w", err)
		}
	}
	if _, err = tx.Exec(ctx, "UPDATE rp_state SET document=$1,revision=revision+1,updated_at=now() WHERE singleton=true", data); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit state: %w", err)
	}
	return nil
}

var _ core.Store = (*Postgres)(nil)
