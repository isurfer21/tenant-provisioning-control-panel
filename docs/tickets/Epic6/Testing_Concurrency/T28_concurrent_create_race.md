# 🛠 GOAL: T28_concurrent_create_race.md.md | 📊 ESTIMATE: 3 hrsh | 🪜 DEPENDS_ON: [T7, T13]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T28: Concurrent Create Race Test 

## Goal
Prove slug uniqueness under parallel creation (Revised Section 16 & Section 5).

## Implementation Instructions (Go/testing goroutines)
Launch two parallel `POST /v1/tenants` using identical slugs (`slug=customer-a`). Wait for both responses and assert exactly one HTTP 201 response with a generated ID, while the other returns HTTP 409 tenant_already_exists.

