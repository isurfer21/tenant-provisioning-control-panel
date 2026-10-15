# 🛠 GOAL: T11_fsm.md.md | 📊 ESTIMATE: 2 hrsh | 🪜 DEPENDS_ON: [T6]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T11: Task State Machine (FSM)

## Goal
Implement task FSM logic strictly enforcing terminal states.

## Implementation Instructions (Go/Logic)
Create `internal/lifecycle/task_fsm.go`. Validations exact to Revised Scope Section 4:
- accepted -> in_progress
- in_progress -> done, failed
Once task status is 'done' or 'failed', no further updates are valid. Return an error if an attempt is made to move away from a terminal state (e.g., trying to update `done` back to something else).

## Acceptance Criteria
- Terminal states correctly lock out backward movement. 

