# Chunk 3 — Persistence Layer (SQLite + Repository Pattern)

**Goal**: Implement `internal/store/store.go` with repository interfaces and `sqlite.go` with a concrete SQLite-backed implementation using raw `database/sql`.

---

## Prompt: Build the store layer

> Create a persistence layer in `internal/store/store.go` and `internal/store/sqlite.go` for Go using standard library `database/sql` with the `modernc.org/sqlite` driver (pure-Go SQLite, no CGo). Use the repository pattern to ensure the API handlers are decoupled from the concrete database.

> Define two repository interfaces in `store.go`:
> ```go
> tenantType interface {
>     CreateTenant(ctx context.Context, t *model.Tenant) error
>     GetTenantByID(ctx context.Context, id string) (*model.Tenant, error)
>     GetTenantBySlug(ctx context.Context, slug string) (*model.Tenant, error) // for uniqueness validation during create.
>     ListTenants(ctx context.Context, filter model.TenantListFilter) ([]*model.Tenant, int64, error) // returns (pageItems, totalCount, error)
>     UpdateTenantStatus(ctx context.Context, tenantID string, newStatus string, version int64) (*model.Tenant, error) // optimistic lock guarded update: UPDATE tenant SET status=$1, version=version+1, updated_at=$3 WHERE id=$4 AND version=$5
>   •  UpdateTenantName(ctx context.Context, tenantID string, newName string, version int64) (*model.Tenant, error)  // parallel UPDATE for name changes with optimistic lock: SET name=$1, version=version+1 where id=$2 and version=$3.
>     DeleteTenant(ctx context.Context, tenantID string) error // logically delete (mark destroyed/failed), not physical delete.
> }

> ```And a TaskStore interface in the same file:
> ```go
> type TaskStore interface {
>     CreateTask(ctx context.Context, t *model.Task) error
>     GetTaskByID(ctx context.Context, id string) (*model.Task, error)
>     ListTasksByTenantID(ctx context.Context, tenantID string) ([]*model.Task, error)
>     UpdateTaskStatus(ctx context.Context, taskID string, newStatus string) error // UPDATE SET status=$1, updated_at=$2 WHERE id=$3 AND status IN ('accepted', 'in_progress') — this prevents updating the status of a terminal task (done or failed), which is our order-tolerance guard.
> }

> The SQLite implementation must support:
> 1. In-memory mode (`file::memory:?cache=shared`) for the default in-process database connection via sql.Open("sqlite", "file::memory:?cache=shared").
>     - Use a single *sql.DB connection pool, not per-request connections.
>     - Create tables on first access: tenants (id TEXT PRIMARY KEY, slug TEXT UNIQUE NOT NULL, name TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'provisioning', version INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL), tasks (id TEXT PRIMARY KEY, type TEXT NOT NULL, tenant_id TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'accepted', created_at TEXT NOT NULL, updated_at TEXT NOT NULL).
>     - Create UNIQUE constraint on `slug` for uniqueness validation. No physical foreign key necessary; application-level referential integrity is sufficient.
> 3. All CRUD operations must use prepared statements with parameterized queries to prevent SQL injection. Never concatenate user input into SQL strings.
> 4. The UpdateTaskStatus method MUST include the optimistic lock guard in its WHERE clause: `WHERE id=$1 AND status IN ('accepted', 'in_progress')`. If zero rows are affected, return a specific error (e.g., `store.ErrTaskUpdateFailed` or check via sql.RowsAffected == 0 and return an error).
> 5. ListTenants should support pagination with `offset` and `limit`, plus optional filtering by `status`. Return total count via a separate COUNT(*) query.
> 6. Add a Close() method to the SQLite implementation that closes the *sql.DB connection.
