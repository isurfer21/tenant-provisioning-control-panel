# 🛠 GOAL: T30_order_of_deliveries.md.md | 📊 ESTIMATE: 4 hrsh | 🪜 DEPENDS_ON: [T23]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T30: Out-of-Order Delivery Test

## Goal
Validate Order-Tolerance logic prevents regressions (Revised Section 13).

## Implementation Instructions (NATS + Integration Testing)
Simulate a scenario where NATS delivers an update status (`in_progress`) AFTER the terminal state (`done`) has successfully been written. Confirm that the in-memory DB state still reflects `done` and the consumer handles the stale event gracefully without rolling back progress to an earlier lifecycle stage. 

