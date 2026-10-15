# Chunk 1 — Project Scaffolding, Docker, & Makefile

**Goal**: Create a runnable Go project skeleton with Docker compose (NATS + service), multi-stage Dockerfile, and a Makefile for single-command boot.

---

## Prompt: Build the foundation

> Create a new Go module called `tenant-provisioning-control-plane` from scratch. Set up the following file structure:
>
> ```
> tenant-provisioning-control-plane/
> ├── go.mod                    # go 1.22+, require all modules
> ├── Makefile                  # Single-command targets:
> │                             #   make up     → docker compose --profile dev up -d && sleep 3 && docker compose --profile app up -d
> │                             #   make test   → go test -race -count=1 ./...
> │                             #   make clean  → docker compose down -v
> ├── docker-compose.yml        # Two services: nats (image: nats:2.10-alpine, port 4222/8222) and app (targeting Dockerfile, depends_on nats, ports 8080→8080)
> │                             # Add healthcheck for both: NATS via ncli info check, app via GET /healthz retry.
> ├── Dockerfile                # Multi-stage: build-stage FROM golang:1.22-alpine with go mod download + go build -o server ./cmd/server; runtime stage FROM alpine:3.19 with COPY server from builder + RUN adduser -D appuser && USER appuser and EXPOSE 8080
> ├── .gitignore                # Binaries, .DS_Store, *.db, .env, vendor/
> └── cmd/
>     ├── server/              # Go module main package
>     │   ├── main.go          # Entry: parse config from env (SERVER_PORT, NATS_URL), init HTTP server with net/http.Mux, attach /healthz handler that returns 200 OK JSON, start serving on port with graceful shutdown via os/signal for SIGINT/SIGTERM.
>     └── worker-sim/          # Go module main package
>         ├── main.go          # Entry: parse CLI flags (--fail-rate, --min-delay-ms, --max-delay-ms, --broker-url), log startup, print ready message. (No NATS integration yet — stub out consumer loop with log.Printf "Worker simulator started. Waiting for tasks...".)
> └── internal/                # Domain logic packages
>     ├── api/                 # HTTP handlers go here
>     ├── broker/              # NATS abstraction goes here
>     ├── idempotency/         # Idempotent consumer logic
>     ├── lifecycle/           # State machine implementation
>     ├── model/               # Domain types
>     └── store/               # Persistence layer
> ```
>
> The Dockerfile should use `ARG` for version info. Add a simple `/healthz` endpoint in the HTTP server that returns `{ "status": "ok" }` with `Content-Type: application/json`.
>
> **Requirements**:
> - No imports beyond stdlib at this stage (no third-party Go modules yet).
> - The Makefile boot command must be exactly one line: `docker compose --profile dev up -d && sleep 3 && docker compose --profile app up -d`.
> - Document in comments how each file will expand.
> - Use go modules, not GOPATH.
