# 🛠 GOAL: T4_config.md.md | 📊 ESTIMATE: 2 hrsh | 🪜 DEPENDS_ON: [T1]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications

# Ticket T4: Configuration Framework

## Goal
Support configuration entirely through environment variables using Go native patterns.

## Implementation Instructions
1. Create `internal/config/config.go` defining a struct:
```go
type Config struct {
    Port        int
    DBPath      string
    NATSURL     string
    LogLevel    string (trace, debug, info, warn, error)
}
func Load() (*Config, error)
```
2. Logic for `Load()` uses `os.Getenv()` with hardcoded defaults:
   - Port: 8080
   - DBPath: ./data/tenant.db
   - NATSURL: nats://localhost:4222
   - LogLevel: INFO

## Acceptance Criteria
- All config variables are injectable via Docker environment mapping.
- Missing critical defaults result in explicit panic or startup error rather than runtime nil pointer exceptions (e.g., missing DBPath causes a startup exit).

