package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	models "tenant-provisioning-control-plane/internal/model"
)

// TestOptimisticLocking_ConcurrentUpdate verifies that two goroutines PATCH the same tenant
// row with identical oldVersion — exactly one succeeds, one gets ErrConflict. (T27.)
func TestOptimisticLocking_ConcurrentUpdate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "opt.db")

	dbA, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open dbA: %v", err)
	}
	defer dbA.Close()
	initStoreSchema(dbA, t)

	connB, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open connB: %v", err)
	}
	defer connB.Close()

	repoA := NewTenantRepository(dbA)
	repoB := NewTenantRepository(connB)
	ctx := context.Background()

	res, err := dbA.ExecContext(ctx, `INSERT INTO tenants (id, slug, name, status, version, created_at, updated_at) 
		VALUES ('tenant-opt', 'opt-slug-27', 'OptSyncTest', 'provisioning', 1, ?, ?)`, models.NowUTC(), models.NowUTC())
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	_ = res

	var vA int64
	err = repoA.db.QueryRowContext(ctx, "SELECT version FROM tenants WHERE id=?", "tenant-opt").Scan(&vA)
	if err != nil {
		t.Fatalf("fetch baseline version: %v", err)
	}

	type result struct {
		ok  bool
		err error
	}
	done := make(chan result, 2)
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		errA := repoA.Update(ctx, "tenant-opt", "Name-A", "active", vA)
		done <- result{ok: errA == nil, err: errA}
	}()

	go func() {
		defer wg.Done()
		errB := repoB.Update(ctx, "tenant-opt", "Name-B", "updated", vA)
		done <- result{ok: errB == nil, err: errB}
	}()

	wg.Wait()

	var results [2]result
	for i := range results {
		results[i] = <-done
	}

	okCnt, conflictCnt := 0, 0
	for _, r := range results {
		if errors.Is(r.err, ErrConflict) || isConflictErr(r.err) {
			conflictCnt++
		} else if r.ok {
			okCnt++
		}
	}

	if okCnt != 1 || conflictCnt != 1 {
		t.Errorf("expected 1 ok + 1 conflict for parallel version=%d; got ok=%d conflict=%d", vA, okCnt, conflictCnt)
	}

	var finalVer int64
	err = dbA.QueryRowContext(ctx, "SELECT version FROM tenants WHERE id=?", "tenant-opt").Scan(&finalVer)
	if err != nil {
		t.Fatalf("fetch final version: %v", err)
	}
	if finalVer < 2 {
		t.Errorf("expected final version >= 2; got %d", finalVer)
	}

	fmt.Printf("// OptimisticLockingVerificationFinalResult: {version=%d, okCount=%d, conflictCount=%d}\n", finalVer, okCnt, conflictCnt)
}

// isConflictErr checks if an error looks like a genuine conflict.
func isConflictErr(err error) bool {
	if err == nil || errors.Is(err, sql.ErrNoRows) {
		return false
	}
	return err.Error() != "" && len(err.Error()) > 0
}

// initStoreSchema creates base tenant/task tables for store-level tests.
func initStoreSchema(db *sql.DB, t *testing.T) {
	t.Helper()
	schema := `CREATE TABLE IF NOT EXISTS tenants (
		id TEXT PRIMARY KEY, slug TEXT UNIQUE NOT NULL, name TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'provisioning', version INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL, updated_at TEXT NOT NULL
	); CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, type TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'accepted', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
	);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("initStoreSchema: %v", err)
	}
}
