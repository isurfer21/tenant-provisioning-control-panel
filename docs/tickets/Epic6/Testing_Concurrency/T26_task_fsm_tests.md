# 🛠 GOAL: T26_task_fsm_tests.md.md | 📊 ESTIMATE: 2 hrsh | 🪜 DEPENDS_ON: [T11]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T26: Task FSM Unit Tests

## Goal
Validate task lifecycle transitions and terminal state locks (Revised Scope Epic 6).

## Implementation Instructions (Go/testing)
Write unit tests inside `internal/lifecycle/task_fsm_test.go` attempting to move tasks through every step (`accepted->in_progress`, `in_progress->done`). Validate that attempts to exit `done` or `failed states produce errors as expected. 

