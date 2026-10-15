# 🛠 GOAL: T29_duplicate_replay.md.md | 📊 ESTIMATE: 2 hrsh | 🪜 DEPENDS_ON: [T22]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T29: Duplicate Event Replay Test

## Goal
Validate Idempotency logic by delivering the same NATS message twice (Revised Section 12).

## Implementation Instructions (NATS + Integration Testing)
Publish a valid task progress event directly to the JetStream stream. Apply business logic, then publish that exact same event again verifying the second application triggers zero database mutations and the `done` status remains single and untouched. 

