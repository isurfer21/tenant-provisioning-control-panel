# 🛠 GOAL: T12_open_task_guard.md.md | 📊 ESTIMATE: 3 hrsh | 🪜 DEPENDS_ON: [T8, T11]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T12: Single Open Task Guard

## Goal
Enforce the "Max 1 open task per tenant" rule from Revised Section 6.

## Implementation Instructions (Go/Service)
This logic resides in the service layer before creating a new task (e.g., handling a PATCH request). 
Before issuing an 'update' or 'destroy' task, query `SELECT COUNT(*) FROM tasks WHERE tenant_id = ? AND status IN ('accepted', 'in_progress')`. If count > 0, reject the request with `409 Conflict` because the open task guard is violated.

## Acceptance Criteria
- Concurrent PATCH requests that attempt to spawn a second open task while one exists must be blocked by this logic before hitting the DB for task creation. 

