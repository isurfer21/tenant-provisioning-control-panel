# 🛠 GOAL: T10_fsm.md.md | 📊 ESTIMATE: 5 hrsh | 🪜 DEPENDS_ON: [T5]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T10: Tenant State Machine (FSM)

## Goal
Implement strict FSM logic for tenant lifecycle transitions.

## Implementation Instructions (Go/Logic)
Create `internal/lifecycle/tenant_fsm.go`. Create a state machine validation engine using the exact mappings in Revised Scope Section 5:
- provisioning -> active, failed
- active       -> updating, destroying
- updating     -> active, failed
- destroying   -> destroyed, failed
- failed       -> destroying

Logic must prevent all invalid transitions and return errors matching the `tenant_update_not_allowed` error contract. 

## Acceptance Criteria
- Implementing a transition to an unlisted state (e.g., provisioning->destroying) MUST panic or fail validation early via `if err != nil { return &ErrInvalidTransition{...} }`.

