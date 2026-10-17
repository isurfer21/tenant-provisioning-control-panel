package tenantprovisioning_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	routerpkg "tenant-provisioning-control-plane/router"
	outboxpkg "tenant-provisioning-control-plane/internal/outbox"
)

// schema matches all tables used across the stack (tenants, tasks + outbox).
const schema = `
CREATE TABLE IF NOT EXISTS tenants(
	id TEXT PRIMARY KEY, slug TEXT UNIQUE NOT NULL, name TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending', version INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS tasks(
	id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, type TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'accepted', created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
` + outboxpkg.Schema

func mustDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil { t.Fatalf("open db: %v", err) }
	if _, err := db.ExecContext(context.Background(), schema); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	return db
}

// ─── Tenant creation (happy path) ✓ covers handler→service→store outbox ──

func TestTenantCreation_HappyPath(t *testing.T) {
	db := mustDB(t); r := routerpkg.New(db, nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/tenants", strings.NewReader(`{"slug":"acme","name":"Acme Corp"}`)))
	if w.Code != 201 { t.Fatalf("wanted 201 got %d: %s", w.Code, w.Body.String()) }

	var resp map[string]any; json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if data["slug"] != "acme" { t.Errorf("slug = %v", data["slug"]) }
	got := data["status"].(string)
	if got != "provisioning" { t.Errorf("status = %s; want provisioning", got) }

	var slug, status string
	err := db.QueryRow("SELECT slug, status FROM tenants WHERE id=?", data["id"]).Scan(&slug, &status)
	if err != nil { t.Fatalf("query tenant: %v", err) }
	if slug != "acme" || status != "provisioning" {
		t.Errorf("db = (%s,%s)", slug, status)
	}
}

// ─── Tenant creation — duplicate slug ✓ covers outbox uniqueness path ──

func TestTenantCreation_DuplicateSlug(t *testing.T) {
	db := mustDB(t); r := routerpkg.New(db, nil)
	pay := `{"slug":"dup-slug","name":"First"}`

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/tenants", strings.NewReader(pay)))
	if w.Code != 201 { t.Fatalf("first: %d (%s)", w.Code, w.Body.String()) }

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/tenants", strings.NewReader(pay)))
	if w.Code < 400 || w.Code > 500 {
		t.Errorf("dup wanted error status got %d (%s)", w.Code, w.Body.String())
	}
}

// ─── Tenant creation — bad request ✓ covers handler validation ──

func TestTenantCreation_InvalidRequest(t *testing.T) {
	db := mustDB(t); r := routerpkg.New(db, nil)
	cases := []struct{ name string; body string }{
		{"empty slug", `{"slug":""}`}, {"missing slug", `{"name":"no-slug"}`}, {"empty body", ""}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/tenants", strings.NewReader(c.body)))
			if w.Code < 400 || w.Code > 499 {
				t.Errorf("want 4xx got %d (%s)", w.Code, w.Body.String())
			}
		})
	}
}

// ─── Get by ID — not found ✓ ──

func TestGetTenantByID_NotFound(t *testing.T) {
	db := mustDB(t); r := routerpkg.New(db, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/v1/tenants/nonexistent-slug-999", nil))
	if w.Code != 404 { t.Errorf("want 404 got %d (%s)", w.Code, w.Body.String()) }
}

// ─── List tenants + pagination ✓ covers service list path ──

func TestListTenants(t *testing.T) {
	db := mustDB(t); r := routerpkg.New(db, nil)
	for _, slug := range []string{"a-a1","a-a2","a-a3"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/tenants", strings.NewReader(`{"slug":"`+slug+`","name":"`+slug+`"}`)))
		if w.Code != 201 { t.Fatalf("seed %s: %d (%s)", slug, w.Code, w.Body.String()) }
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/v1/tenants?page=1&limit=2", nil))
	if w.Code != 200 { t.Fatalf("list: got %d (%s)", w.Code, w.Body.String()) }
	var resp map[string]any; json.Unmarshal(w.Body.Bytes(), &resp)
	list := resp["data"].([]any)
	if len(list) != 2 { t.Errorf("page1 length = %d; want 2", len(list)) }
}

// ─── Full lifecycle: create→update→verify DB state ✓ covers service.updateTenant + optimistic lock ──

func TestTenantFullLifecycle(t *testing.T) {
	db := mustDB(t); r := routerpkg.New(db, nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/tenants", strings.NewReader(`{"slug":"lc-lc","name":"Lc"}`)))
	if w.Code != 201 { t.Fatalf("create: %d (%s)", w.Code, w.Body.String()) }
	var crResp map[string]any; json.Unmarshal(w.Body.Bytes(), &crResp)
	tenantID := crResp["data"].(map[string]any)["id"].(string)

	w = httptest.NewRecorder()
	pay, _ := json.Marshal(map[string]string{"name": "NewName"})
	r.ServeHTTP(w, httptest.NewRequest("PATCH", "/v1/tenants/"+tenantID, strings.NewReader(string(pay))))
	if w.Code != 200 { t.Fatalf("update: want 200 got %d (%s)", w.Code, w.Body.String()) }

	var resp map[string]any; json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["data"].(map[string]any)["name"].(string) != "NewName" {
		t.Error("name not updated")
	}

	var name, status string; var ver int64
	db.QueryRow("SELECT name,status,version FROM tenants WHERE id=?", tenantID).Scan(&name, &status, &ver)
	if name != "NewName" { t.Errorf("db name = %s", name) }
	if status != "provisioning" { t.Errorf("db status = %s", status) }
	if ver != 2 { t.Errorf("db version = %d; want 2", ver) }
}

// ─── Outbox events (written to DB on tenant create — no NATS needed) ✓ ──

func TestTenantCreate_WritesOutboxEvent(t *testing.T) {
	db := mustDB(t); r := routerpkg.New(db, nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/tenants", strings.NewReader(`{"slug":"ob-test-1","name":"Ob"}`)))
	if w.Code != 201 { t.Fatalf("create: %d (%s)", w.Code, w.Body.String()) }

	var count int; db.QueryRow("SELECT COUNT(*) FROM outbox_events").Scan(&count)
	if count != 1 { t.Fatalf("outbox rows = %d; want 1", count) }

	var subject string
	db.QueryRow("SELECT subject FROM outbox_events ORDER BY id LIMIT 1").Scan(&subject)
	if len(subject) == 0 { t.Error("empty subject") }

	var tenantStatus string
	db.QueryRow("SELECT status FROM tenants WHERE slug='ob-test-1'").Scan(&tenantStatus)
	if tenantStatus != "provisioning" { t.Errorf("tenant = %s; want provisioning", tenantStatus) }

	// Verify linked task row also exists.
	var taskCount int
	var tenantID string
	db.QueryRow("SELECT id FROM tenants WHERE slug='ob-test-1'").Scan(&tenantID)
	db.QueryRow("SELECT COUNT(*) FROM tasks WHERE tenant_id=?", tenantID).Scan(&taskCount)
	if taskCount != 1 { t.Errorf("linked tasks = %d; want 1", taskCount) }
}

// ─── StartDestroy → GAP 4 (outbox + status change) ✓ ──

func TestTenantDelete_StartDestroy(t *testing.T) {
	db := mustDB(t); r := routerpkg.New(db, nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/tenants", strings.NewReader(`{"slug":"del-tenant","name":"Del"}`)))
	if w.Code != 201 { t.Fatalf("create: %d (%s)", w.Code, w.Body.String()) }
	var crResp map[string]any; json.Unmarshal(w.Body.Bytes(), &crResp)
	tenantID := crResp["data"].(map[string]any)["id"].(string)

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", "/v1/tenants/"+tenantID, nil))
	if w.Code != 200 { t.Fatalf("destroy: want 200 got %d (%s)", w.Code, w.Body.String()) }

	var drResp map[string]any; json.Unmarshal(w.Body.Bytes(), &drResp)
	if drResp["data"].(map[string]any)["status"].(string) != "destroying" {
		t.Error("tenant not destroyed")
	}

	var count int; db.QueryRow("SELECT COUNT(*) FROM outbox_events").Scan(&count)
	if count != 2 { t.Errorf("outbox count = %d; want 2 (create+destroy)", count) }
}
