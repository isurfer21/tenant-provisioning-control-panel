# 🛠 GOAL: T3_migrations.md.md | 📊 ESTIMATE: 3 hrsh | 🪜 DEPENDS_ON: [None]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications

# Ticket T3: Database Migrations & Schema Setup

## Goal
Create persistent schema and migration runner.

## Implementation Instructions
1. Migrations directory at `migrations/` containing raw SQL files since we don't want external migration tools for a take-home assignment:
   - `001_init.sql`: Creates tables based on Revised Scope Section 17 exactly.
2. Schema to create strictly:
```sql
CREATE TABLE IF NOT EXISTS tenants (
    id TEXT PRIMARY KEY,
    slug TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'provisioning',
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'accepted',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS outbox_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    aggregate_id TEXT NOT NULL,
    payload TEXT NOT NULL,
    published INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS processed_events (
    event_id TEXT PRIMARY KEY,
    processed_at TEXT NOT NULL
);
```
3. Go migration helper (`internal/migrate/migrator.go`) that reads and executes SQL files in alphabetical order inside `db.Open()`. It must use a transaction for safety.

## Deliverables
- Migrations folder with schema exactly matching the Revised Scope Table requirements.
- Automatic running of these migrations inside `main.go` startup logic.

## Acceptance Criteria
- First run creates tables seamlessly. Subsequent runs do not throw errors (idempotent via IF NOT EXISTS or migration table).

