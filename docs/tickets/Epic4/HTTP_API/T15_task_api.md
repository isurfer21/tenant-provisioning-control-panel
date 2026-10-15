# 🛠 GOAL: T15_task_api.md.md | 📊 ESTIMATE: 3 hrsh | 🪜 DEPENDS_ON: [T8, T11, T16]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T15: Task API Endpoints (Gin Framework)

## Goal
Build read-only task tracking endpoints matching Revised Scope Section 8.

## Implementation Instructions (Go/Gin)
Implement handlers in `internal/api/task_handler.go`:
- `GET /v1/tasks/{taskId}` -> Query tasks table, return task or `404`. 
- `GET /v1/tenants/{tenantId}/tasks` -> Filter by tenantID and optional status/type. Return paginated list. Strictly READ-ONLY (no POST/PUT allowed).

## Acceptance Criteria
- Tasks are strictly read-only over HTTP. Workers only modify them via NATS messages, never via HTTP endpoints. 

