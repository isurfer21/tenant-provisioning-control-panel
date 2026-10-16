package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	models "tenant-provisioning-control-plane/internal/model"
)

// TestConcurrentCreateRace_ConcurrentInsertes exercises the repo.Create UNIQUE slug constraint.
func TestConcurrentCreateRace_ConcurrentInserts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "race.db")

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
	sameSlug := "conc-same"

	type result struct {
		ok  bool
		err error
	}
	var r1, r2 result
	done := make(chan struct{}, 2)
	wg := sync.WaitGroup{}
	wg.Add(2)

	barrier := make(chan struct{})

	go func() {
		defer wg.Done()
		<-barrier // synchronize: both enter at the same instant
		tok1 := &models.Tenant{Slug: sameSlug, Name: "RaceTest-A"}
		errA := repoA.Create(ctx, tok1)
		r1.ok = errA == nil
		done <- struct{}{}
	}()

	go func() {
		defer wg.Done()
		<-barrier // synchronize: both enter at the same instant
		tok2 := &models.Tenant{Slug: sameSlug, Name: "RaceTest-B"}
		errB := repoB.Create(ctx, tok2)
		r2.ok = errB == nil
		done <- struct{}{}
	}()

	close(barrier) // release both goroutines simultaneously
	wg.Wait()

	okCnt, conflictCnt := 0, 0
	for _, r := range []result{r1, r2} {
		if r.ok {
			okCnt++
		} else if errors.Is(r.err, ErrAlreadyExists) || isUniqConstraintErr(r.err) {
			conflictCnt++
		}
	}

	// Exactly one success expected; the other may conflict or be silently skipped.
	if okCnt != 1 {
		t.Errorf("expected exactly 1 OK for slug=%q; got ok=%d conflict=%d", sameSlug, okCnt, conflictCnt)
	}

	var count int
	err = dbA.QueryRowContext(ctx, "SELECT COUNT(*) FROM tenants WHERE slug=?", sameSlug).Scan(&count)
	if err != nil {
		t.Fatalf("final row count for slug=%s: %v", sameSlug, err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 tenant row; got %d", count)
	}

	fmt.Printf("// ConcurrentCreateTestResult: ok=%d conflicts=%d verified=true\n", okCnt, conflictCnt)
}

// isUniqConstraintErr checks if an error is a UNIQUE constraint violation.
func isUniqConstraintErr(err error) bool {
	if err == nil {
		return false
	}
	e := err.Error()
	return strings.Contains(e, "UNIQUE") || strings.Contains(e, "constraint")
}


