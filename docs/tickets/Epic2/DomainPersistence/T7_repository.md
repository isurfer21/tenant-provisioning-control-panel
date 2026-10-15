# 🛠 GOAL: T7_repository.md | 📊 ESTIMATE: 4h | 🪜 DEPENDS_ON: [T3_migrations, T5_tenant]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T7: Tenant Repository

## Goal
Implement persistence CRUD operations with UNIQUE constraint enforcement matching Revised section 5 & 17.

## Implementation Instructions (SQLite + modernc.org/sqlite)
Create `internal/store/tenant_repository.go` with `sql.DB` bindings. 
Methods to implement:
- `Create(ctx, t *model.Tenant) error`: INSERT INTO tenant table. If slug violates UNIQUE constraint (`ErrCode == "UNIQUE constraint failed"` in SQL driver), return a specific repository error that the Gin API can map to `tenant_already_exists`.
- `GetByID(ctx, id string)` -> `(*Tenant, ErrNotFound)`
- `GetBySlug(ctx, slug string)` -> `(Tenant, err)`: Must check against ALL statuses because Revised scope says "Unique forever, including destroyed tenants" (so do not rely solely on soft-deletes unless we explicitly model the outbox). Just query full table for uniqueness.
- `Update(ctx, id string, newName string, newStatus string, oldVersion int64) error`: Use Optimistic locking. The query syntax: 
`UPDATE tenants SET name = ?, status = ?, version = version + 1, updated_at = ? WHERE id = ? AND version = ?`.
Return an error if `RowsAffected() == 0`.

## Acceptance Criteria
- Concurrent creates attempting to insert the same slug MUST guarantee exactly one success by utilizing DB-level UNIQUE constraints.
