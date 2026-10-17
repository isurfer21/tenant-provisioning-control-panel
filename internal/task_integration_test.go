package tenantprovisioning_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	routerpkg "tenant-provisioning-control-plane/router"
	outboxpkg "tenant-provisioning-control-plane/internal/outbox"
)

// taskSchema is the subset we need: tenants + tasks (no outbox for these tests).
const taskSchema = `
CREATE TABLE IF NOT EXISTS tenants(
	id TEXT PRIMARY KEY, slug TEXT UNIQUE NOT NULL, name TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending', version INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS tasks(
	id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, type TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'accepted', created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
` + outboxpkg.Schema

func taskDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil { t.Fatalf("open db: %v", err) }
	if _, err := db.ExecContext(context.Background(), taskSchema); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	return db
}

// ─── TaskGetByID returns 404 when no such task exists ✓ covers handler.getTask + service + store repo ──

func TestTaskGetByID_NotFound(t *testing.T) {
	db := taskDB(t); r := routerpkg.New(db, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/v1/tasks/non-existent-task-uuid-here", nil))
	if w.Code != 404 { t.Errorf("want 404 got %d (%s)", w.Code, w.Body.String()) }
}

// ─── Task linked on tenant create: count = 1 ✓ covers outbox.CreateTenantTaskWithOutbox path ──

func TestTaskCreatedOnTenantLink(t *testing.T) {
	db := taskDB(t); r := routerpkg.New(db, nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/tenants", strings.NewReader(`{"slug":"tl-tenant","name":"TL"}`)))
	if w.Code != 201 { t.Fatalf("create: %d (%s)", w.Code, w.Body.String()) }

	var tenantResp map[string]any; json.Unmarshal(w.Body.Bytes(), &tenantResp)
	tenantID := tenantResp["data"].(map[string]any)["id"].(string)

	// Verify exactly one linked deploy task exists for this tenant.
	var count int; db.QueryRow("SELECT COUNT(*) FROM tasks WHERE tenant_id=?", tenantID).Scan(&count)
	if count != 1 { t.Fatalf("linked tasks for tenant = %d; want 1", count) }

	// Task status should be 'accepted' (default from outbox path).
	var taskStatus string
	db.QueryRow("SELECT status FROM tasks WHERE tenant_id=? LIMIT 1", tenantID).Scan(&taskStatus)
	if taskStatus != "accepted" { t.Errorf("task status = %s; want accepted", taskStatus) }
}

// ─── DeleteStartDestroy creates destroy-type outbox event + task ✓ covers service.StartDestroy → outbox.CreateDestroyTaskWithOutbox ──

func TestDestroy_OutboxEventAndTaskCreated(t *testing.T) {
	db := taskDB(t); r := routerpkg.New(db, nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/tenants", strings.NewReader(`{"slug":"dst-tenant","name":"Destroy"}`)))
	if w.Code != 201 { t.Fatalf("create: %d (%s)", w.Code, w.Body.String()) }
	var crResp map[string]any; json.Unmarshal(w.Body.Bytes(), &crResp)
	tID := crResp["data"].(map[string]any)["id"].(string)

	// DELETE → should create destroy task + outbox event.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", "/v1/tenants/"+tID, nil))
	if w.Code != 200 { t.Fatalf("destroy: want 200 got %d (%s)", w.Code, w.Body.String()) }

	// Verify outbox row created for destroy.
	var obsCount int; db.QueryRow("SELECT COUNT(*) FROM outbox_events").Scan(&obsCount)
	if obsCount != 2 { t.Errorf("outbox count = %d; want 2 (create+destroy)", obsCount) }

	// Verify destroy task exists.
	var destTasks int; db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE tenant_id=? AND type='destroy'`, tID).Scan(&destTasks)
	if destTasks != 1 { t.Fatalf("destroy task = %d; want 1", destTasks) }

	// Tenant status should be 'destroying.
	var tStatus string; db.QueryRow("SELECT status FROM tenants WHERE id=?", tID).Scan(&tStatus)
	if tStatus != "destroying" { t.Errorf("tenant status = %s; want destroying", tStatus) }
}
