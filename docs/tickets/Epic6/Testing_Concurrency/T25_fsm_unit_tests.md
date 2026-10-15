# 🛠 GOAL: T25_fsm_unit_tests.md.md | 📊 ESTIMATE: 3 hrsh | 🪜 DEPENDS_ON: [T10]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T25: Tenant FSM Unit Tests

## Goal
Comprehensive valid/invalid transition tests (Revised Scope Epic 6).

## Implementation Instructions (Go/testing)
Use Go table-driven test cases inside `internal/lifecycle/tenant_fsm_test.go`. Test every single legal transition (`provisioning->active`, etc.). Also implement explicit negative tests attempting invalid moves (e.g. provisioning -> destroyed) verifying the returned error is not nil and matches the expected FSM rejection logic. 

