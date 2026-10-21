# Demo Scripts — Tenant Provisioning Control Plane

Interactive shell demos that walk through the full system live. Each script makes real HTTP calls against a running deployment and inspects internal state (SQLite, NATS) for ground-truth evidence.

Prerequisites

- Docker & Docker Compose
- curl, python3
- sqlite3 (for direct DB inspection — optional)

Quick start

    ./demo/01-start-infrastructure.sh    # Builds & boots API + NATS + worker-sim
    ./demo/demo-all.sh                   # Runs 02–07 automatically in order

Or run each demo manually in sequence:

    1. Infrastructure         Demo   What you see                                            Scripts
    ──────────────── ───────── ─────────────────────────────────────────────────────────── ─────────────────────
    NATS + API + SIM         Bootstrap     `docker compose up` — all three services           01-start-infrastructure.sh
    Tenant CRUD lifecycle    Create / Update (name & status) / Delete with                02-tenant-lifecycle-demo.sh
                             optimistic locking, FSM validation inline

    Error paths             All HTTP error codes: 400, 404, 409, 422 with real        03-error-handling-validation-demo.sh
                          payloads and responses

    Task API & outbox       Tenant creation triggers task + outbox event atomically;     04-task-api-demo.sh
                             worker-sim picks it up via NATS in ~5–15 s

    Outbox → Worker flow   End-to-end async pipeline: create tenant, watch outbox         05-outbox-worker-lifecycle-demo.sh
                          event reach NATS → worker processes → task completes — live
                          polling with real-time status updates

    FSM state machines     Valid & invalid transitions for both Tenant FSM and           06-fsm-state-machine-demo.sh
                          Task FSM, plus open-task guard

    Architecture overview   Project layout, component diagram, data model                07-architecture-overview-demo.sh
                          reference, API spec table, design patterns, error map

Each script is self-contained: it starts clean (creates a fresh tenant), walks through
the flow with verified HTTP status codes, and leaves the system running so you can poke
around at $API_URL and the SQLite DB.

To tear everything down:

    cd ..                                    # back to repo root
    docker compose down --remove-orphans
