# 🛠 GOAL: T23_order_tolerant_guard.md.md | 📊 ESTIMATE: 3 hrsh | 🪜 DEPENDS_ON: [T22]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T23: Order-Tolerant Processing Guard

## Goal
Ensure state never regresses (Revised Section 13 & 23).

## Implementation Instructions (Go/DB+FSM)
When applying an inbound progress update, verify against the current state of the task before updating. The business logic in `lifecycle/task_fsm.go` strictly prevents backward transitions (`done → in_progress`). 

For example: if the incoming progress message says "in_progress" but the DB already has status = "done", reject it with a logged warning instead of applying it.

## Acceptance Criteria
- Out-of-order messages (e.g., stale `accepted` arriving after terminal states) cannot change current state, preventing regressions while still keeping the stream clean. 

