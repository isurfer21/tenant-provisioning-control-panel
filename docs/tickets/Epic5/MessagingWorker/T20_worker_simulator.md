# 🛠 GOAL: T20_worker_simulator.md.md | 📊 ESTIMATE: 5 hrsh | 🪜 DEPENDS_ON: [T4, T6]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T20: Worker Simulator Process

## Goal
Simulate worker provisioning via CLI matching Revised Section 15.

## Implementation Instructions (Go/CLI)
Construct a standalone CLI process (`cmd/worker-sim/main.go`) using `flag.String("fail-rate")` etc. logic from Revised Scope:
- Connects to NATS JetStream, subscribes to `tasks.create.*` stream.
- On receiving a task event: sleep for a random duration between `min-delay-ms` and `max-delay-ms`.
- Roll a dice if `rand.Float64() < fail-rate` to simulate failure; otherwise succeed.
- Push an update via NATS (using the broker package) pushing `in_progress`, then `done` or `failed` back out as progress updates.

## Acceptance Criteria
- CLI flags `--fail-rate`, `--min-delay-ms`, `--max-delay-ms` fully control task simulation behavior deterministically for testing purposes. 

