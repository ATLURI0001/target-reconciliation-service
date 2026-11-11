package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ATLURI0001/resilienceplane/internal/core"
)

func TestMemoryTransactionalContract(t *testing.T) { testContract(t, NewMemory()) }
func testContract(t *testing.T, s core.Store) {
	t.Helper()
	ctx := context.Background()
	token := fmt.Sprintf("test-%d", time.Now().UnixNano())
	if err := s.Update(ctx, func(st *core.State) error { st.Clusters[token] = core.Cluster{ID: token}; return nil }); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("abort")
	if err := s.Update(ctx, func(st *core.State) error { delete(st.Clusters, token); return sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("callback error: %v", err)
	}
	view, err := s.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := view.Clusters[token]; !ok {
		t.Fatal("callback failure committed")
	}
	delete(view.Clusters, token)
	view, err = s.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := view.Clusters[token]; !ok {
		t.Fatal("snapshot aliases committed state")
	}
	var captured *core.State
	if err = s.Update(ctx, func(st *core.State) error { captured = st; return nil }); err != nil {
		t.Fatal(err)
	}
	delete(captured.Clusters, token)
	view, err = s.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := view.Clusters[token]; !ok {
		t.Fatal("callback reference aliases committed state")
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if err = s.Update(ctx, func(st *core.State) error { t.Fatal("cancelled callback executed"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled update: %v", err)
	}
}
func TestMemoryConcurrentUpdates(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := s.Update(ctx, func(st *core.State) error { k := fmt.Sprint(i); st.Clusters[k] = core.Cluster{ID: k}; return nil }); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	st, err := s.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Clusters) != 50 {
		t.Fatalf("lost writes: %d", len(st.Clusters))
	}
}
func TestMemoryAuditAppendOnly(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()
	if err := s.Update(ctx, func(st *core.State) error {
		st.Audit = append(st.Audit, core.Audit{Sequence: 1, At: time.Now().UTC(), Actor: "test", Action: "create"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(ctx, func(st *core.State) error { st.Audit[0].Actor = "forged"; return nil }); !errors.Is(err, ErrAuditMutation) {
		t.Fatalf("audit mutation: %v", err)
	}
	if err := s.Update(ctx, func(st *core.State) error { st.Audit = nil; return nil }); !errors.Is(err, ErrAuditMutation) {
		t.Fatalf("audit truncation: %v", err)
	}
}
func TestPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required; database integration not executed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s, err := NewPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	testContract(t, s)
	// All test rows are additive: no destructive cleanup of arbitrary databases.
	id := fmt.Sprintf("persistent-%d", time.Now().UnixNano())
	err = s.Update(ctx, func(st *core.State) error {
		st.Clusters[id] = core.Cluster{ID: id}
		st.Audit = append(st.Audit, core.Audit{Sequence: len(st.Audit) + 1, At: time.Now().UTC(), Actor: "integration", Action: "register", Resource: id, Result: "ok"})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	st, err := second.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Clusters[id]; !ok {
		t.Fatal("state did not survive reconnect")
	}
	var count int
	if err = s.pool.QueryRow(ctx, "SELECT count(*) FROM rp_audit WHERE resource=$1", id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("audit mirror count: %d", count)
	}
	if _, err = s.pool.Exec(ctx, "UPDATE rp_audit SET actor='forged' WHERE resource=$1", id); err == nil {
		t.Fatal("audit SQL mutation was accepted")
	}
	// Two independently connected repositories must not lose concurrent writes.
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var repository core.Store = s
			if i%2 == 1 {
				repository = second
			}
			key := fmt.Sprintf("%s-concurrent-%d", id, i)
			if err := repository.Update(ctx, func(st *core.State) error { st.Clusters[key] = core.Cluster{ID: key}; return nil }); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	st, err = second.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 16; i++ {
		if _, ok := st.Clusters[fmt.Sprintf("%s-concurrent-%d", id, i)]; !ok {
			t.Fatalf("concurrent update %d lost", i)
		}
	}
}
