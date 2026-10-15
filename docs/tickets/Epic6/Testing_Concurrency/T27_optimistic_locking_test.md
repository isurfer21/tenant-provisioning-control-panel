# 🛠 GOAL: T27_optimistic_locking_test.md.md | 📊 ESTIMATE: 3 hrsh | 🪜 DEPENDS_ON: [T9, T14]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T27: Optimistic Locking Concurrency Test

## Goal
Prove exact concurrency correctness under race conditions (Revised Section 16).

## Implementation Instructions (Go/testing with goroutines)
Create a parallel test launching two simultaneous HTTP PATCH requests against the exact same tenant resource sending the same `version=3` in the API request. Assert exactly one response returns 200 OK and the second returns 409 Conflict via an async wait group / channel mechanism. 

