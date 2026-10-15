# 🛠 GOAL: T8_task_repo.md | 📊 ESTIMATE: 3h | 🪜 DEPENDS_ON: [T3_migrations, T6_task]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T8: Task Repository

## Goal
Implement storage and retrieval logic for asynchronous tasks matching Revised Section 17.

## Implementation Instructions (SQLite)
Create `internal/store/task_repository.go`. 

Required methods:
- `Insert(ctx, t *model.Task)`
- `GetByID(ctx, id string)`
- `ListByTenantID(tenantID, filters) ([]Task, error)`: Support filtering by status/type and pagination offset.
- `UpdateStatus(ctx, taskID string, newStatus string)`: 
`UPDATE tasks SET status = ?, updated_at = ? WHERE id = ? AND status IN ('accepted', 'in_progress')`. 
Crucial: Include the `status IN (...)` guard to strictly prevent updating terminal states (done/failed), enforcing order-tolerance.

## Acceptance Criteria
- Updating a terminal task (Status='done') via this method MUST fail / affect 0 rows if called again because of the status whitelist in the WHERE clause. 
