# 🛠 GOAL: T2_docker.md.md | 📊 ESTIMATE: 3 hrsh | 🪜 DEPENDS_ON: [T1]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications

# Ticket T2: Docker Compose Environment

## Goal
Configure Docker Compose to bring up the entire ecosystem (NATS JetStream, SQLite volume, API + Worker).

## Implementation Instructions
1. `docker-compose.yml` with the following exact services:
   - **nats**: Image `nats:2.10-alpine`. Expose 4222/8222. Map a named volume `nats_data` to ensure JetStream streams persist on restart. Add flags for cluster mode if needed, otherwise single node JetStream is fine.
   - **postgres** (Optional, but Revised scope says SQLite persistent volume so we MUST use a volume for the SQLite file!): Volume `sqlite_data:/var/lib/sqlite/data`. Mount to `/app/data` in container.
   - **api**: Target your Dockerfile. Depends on nats. ENV vars: `PORT=8080`, `DB_PATH=/app/data/tenant.db`, `NATS_URL=nats://nats:4222`. Port 8080 exposed.
   - **worker-sim**: Target the worker binary or same Dockerfile with CMD override. Depends on nats. ENV vars: `BROKER_URL=nats://nats:4222`, `FAIL_RATE=0.2`, `MIN_DELAY_MS=500`, `MAX_DELAY_MS=3000`.

## Deliverables
- `docker-compose.yml` containing exactly these four services.
- Volumes configured such that restarting the system (`docker compose down` then `up`) still has NATS JetStream streams and SQLite data intact.
- `make boot` orchestrates everything in one line.

## Acceptance Criteria
- `docker compose up` brings up complete ecosystem with minimal logs (no warnings).
- Data survives a full stop/start (`docker rm -f`, `docker compose up`).

