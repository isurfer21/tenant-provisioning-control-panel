# 🛠 GOAL: T1_bootstrap.md.md | 📊 ESTIMATE: 4 hrsh | 🪜 DEPENDS_ON: [None]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications

# Ticket T1: Bootstrap Go Project (Gin Framework)

## Goal
Create the repository structure, build system, and Gin-based application foundation.

## Implementation Instructions (Go/Gin)
1. Initialize module: `go mod init tenant-provisioning-control-plane`.
2. Dependencies: `gin-gonic/gin` for HTTP framework, `github.com/joho/godotenv` for optional env loading, `github.com/google/uuid`.
3. Project Structure: 
   - `cmd/server/main.go`: Entry point, initializes Gin router, loads config, starts server.
   - `cmd/worker-sim/main.go`: Worker simulator entry point.
   - `internal/model/*`
   - `internal/store/*`
   - `Makefile`
4. Makefile must expose:
   - `make boot`: builds both images via Docker Compose (`docker compose up`), and health-check loop until `/healthz` returns.
   - `make test`: Runs `go test -race ./...`.

## Deliverables
- A clean `cmd/` and `internal/` tree fully typed in Go (Gin framework).
- Multi-stage Dockerfile for the API service (`golang:1.23-alpine` builder -> `alpine:3.20` runtime).
- Makefile with single-command booting capability.

## Acceptance Criteria
- Running `make boot` builds the Go binary and starts Gin successfully in Docker.
- The `/healthz` endpoint returns `{ "status": "ok" }`.

