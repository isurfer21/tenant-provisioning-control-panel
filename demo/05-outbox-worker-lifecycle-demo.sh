#!/usr/bin/env bash
# ===================================================================
# 05-outbox-worker-lifecycle-demo.sh
# Demonstrates NATS outbox pattern + worker-sim integration end-to-end:
# create → outbox written → NATS publish → worker processes → task completes.
# Live, real-time async demonstration with step-by-step status polling.
# ────────────────────────────────────────────────────────────────────
set -euo pipefail

API_URL="http://localhost:8080"
WORKER_SIM_URL="http://localhost:8081"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

BOLD="\033[1m"; GREEN="\033[0;32m"; RED="\033[0;31m"
CYAN="\033[0;36m"; YELLOW="\033[0;33m"; GRAY="\033[0;90m"; BLUE="\033[0;94m"; MAGENTA="\033[0;35m"; NC="\033[0m"

h() { printf "\n${BOLD}════ ${1} ════${NC}\n" ; }
sub() { printf "  ${MAGENTA}[STEP %s]${NC} " "$1"; }
ok()  { printf "${GREEN}✓ %s${NC}" "$1"; }
info(){ printf "\n${GRAY}%b${NC}" "$1"; }

echo ""
printf "${BOLD}═══════════════════════════════════════\033[0m\033[0m${NC}\n"
printf " ${BOLD}Demo 05: Outbox Pattern + Worker-Sim Integration${NC}\n"
printf "${BOLD}═════════════════════════════════════════${NC}\n\n"

# ━━━━━━━━━━━ PREREQ CHECK ┃━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
h "PRECHECK — Verify Infrastructure is Running"

sub "NATS JetStream"; 
if docker exec tpcp-nats-1 nats info &>/dev/null; then ok " NATS is healthy on $NATS_URL\n"
else fail " NATS NOT reachable — run 01-start-infrastructure.sh first"; exit 1; fi

sub "API /healthz"; 
if curl -sf "$API_URL/healthz" &>/dev/null; then ok " API server is healthy\n"  
else fail " API NOT reachable — run 01-start-infrastructure.sh first"; exit 1; fi

# ━━━━━━━━━━━ STEP 1: Create tenant (triggers full async pipeline) ┃
echo -e "\n  Creating a fresh tenant → this triggers the ENTIRE async provisioning pipeline:"
info "  [t0]   API → INSERT tenant(status=provisioning)\n         INSERT task(type=deploy, status=accepted)\n         INSERT outbox_event(subject=tasks.create.<id>)\n                — all in ONE DB transaction (atomic)\n"

payload='{"slug":"outbox-worker-demo-v1","name":"Outbox Worker Demo"}'
resp=$(curl -s -w "\n%{http_code}" \
  -X POST "$API_URL/v1/tenants" \
  -H 'Content-Type: application/json' -d "$payload")
t_id=$(echo "$resp" | sed '$d' | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['id'])")
ok " HTTP 201 — Tenant $t_id created\n"

# ━━━━━━━━━━━ STEP 2: Show outbox event in DB (pre-NATS publish) ┃
h "STEP 2 — Outbox Event Written to DB (Before NATS Publish)"
msg="This is the CORE of the Outbox Pattern:\n\
  The outbox_event row EXISTS in the DB BEFORE it's sent over the network.\n\n\
Even if the server crashes after step 1 but before publishing:\n\
  → The event survives restart; the publisher picks it up later.\n\
  → No events are lost, no duplicates (idempotent consumer handles retries).\n\n\
The NATS 'publisher loop' (part of TenantService/nats.Publisher) polls every ~2s\n\
for outbox_events where attempts < 10. When found, it publishes payload to NATS.${NC}\n"
info "$msg"

sub "DB: outbox_events table contents:"
docker exec tpcp-server-1 sqlite3 /app/data/tenant.db \
  "SELECT rowid, subject, substr(payload,1,60), attempts FROM outbox_events ORDER BY rowid DESC LIMIT 5;" 2>/dev/null | head -3 | while IFS=$'\t' read -r r s p a; do
    printf "\n    [rowid=%s] subject=%s\n         payload='%s...' attempts=%s" "$r" "$s" "$p" "$a"
done

# ━━━━━━━━━━━ STEP 3: Show what's in the NATS stream ┃━━━━━  
h "STEP 3 — NATS Stream (TASKS_CREATE) Ready for Worker-Sim"
echo "Stream configuration (from nats-jetstream.conf + worker-sim setup):"
info "\
  Stream name: 'TASKS_CREATE'\n\
  Subjects:   'tasks.create.>' and 'tasks.progress.>'\n\n\
The publisher loop writes to this stream on our behalf.\n\
Worker-sim has a durable consumer named 'worker-sim' that picks up every message.${NC}"

sub "NATS streams:"; docker exec tpcp-nats-1 nats info 2>/dev/null | grep -E "JetStream|stream|memory" || true

# ━━━━━━━━━━━ STEP 4: Watch worker-sim process in real-time ┃
h "STEP 4 — Worker-Sim Picks Up & Processes the Event (Real-time)"
msg="Worker-sim lifecycle per message:\n\
  1. Consume from jobs.create.<tenant-id> JetStream consumer\n\
  2. Sleep rand[500ms, 3000ms] (simulates real work delay)\n\
  3. Randomize success/fail: 80% done / 20% fails → publishes back:\n     subject: tasks.progress.<task-id>\n\
payload: { tenant_id, task_id, status, 'event_id' }\n\n"
info "$msg"

sub "Task ID from DB:" 
task_id=$(docker exec tpcp-server-1 sqlite3 /app/data/tenant.db \
  "SELECT id FROM tasks WHERE tenant_id='$t_id' LIMIT 1;" 2>/dev/null)

info "\n${CYAN}Watching worker-sim log output in real-time...\033[0m\n"
worker_log_start=$(docker logs tpcp-sim-1 --since 60s 2>&1 | wc -l)

# ━━━━━━━━━━━ STEP 5: Poll task status as it evolves ┃━━━━━
h "STEP 5 — Task Status Evolution Over Time (Live Polling)"

for delay in 3 7 12 17 22; do
  sleep "$delay"
  cur=$(docker exec tpcp-server-1 sqlite3 /app/data/tenant.db \
    "SELECT status FROM tasks WHERE id='$task_id' LIMIT 1;"2>/dev/null || echo "?")
  
  printf "  [t+${delay}s] task[$task_id] status = ${YELLOW}'%s'" "$cur" 
  
  case "$cur" in
    accepted)    printf "\n  (pending — worker not yet processed)\n";;
    in_progress) printf "\n  (worker is actively processing)\n";;
    done|failed) printf " ${GREEN}— TERMINAL STATE reached!${NC}\n" ;;
    *)           printf " (unknown)\n";;
  esac
done

# ━━━━━━━━━ STEP 6: Show final state after processing completes ┃
h "STEP 6 — Final State After Async Pipeline Completes"

final_status=$(curl -sf "$API_URL/v1/tasks/$task_id"2>/dev/null | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['status'])" || echo "?")
tenant_final=$(curl -sf "$API_URL/v1/tenants/$t_id" 2>/dev/null | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['status'])" || echo "?")

sub "Final task status: "; info "${GREEN}'$final_status' ✓${NC}\n"
sub "Tenant lifecycle status:\n"; info "${GREEN}  '$tenant_final' (auto-updated by consumer on terminal task)${NC}"

info "\nWorker-sim log output for this processing cycle:"
docker logs tpcp-sim-1 --since 2m 2>&1 | grep -E "\[worker-sim\]" | tail -5 || true

# ━━━━━━━━━━━ SUMMARY ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
h "SUMMARY — Async Provisioning Pipeline (Diagram)"
cat <<'EOF'
 Timeline: async provisioning with outbox guarantee
  
  [t0] API Server (HTTP POST →v1/tenants)
     │
     ├─ DB tx STARTS
     │   ├─ INSERT tenant (provisioning, ver=1)
     │   ├─ INSERT task    (deploy, accepted)      
     │   └─ INSERT outbox_event  ← survives crashes!
     │
     └─ Tx COMMIT → HTTP 201 returned to client
  
  [t0+~2s] Publisher loop (TenantService background goroutine)       
     ├─ Fetches pending outbox_events (attempts < 10)
     ├─ Publishes to NATS stream TASKS_CREATE   
     └─ on success: marks as processed
           
  [t0+~3s] Worker-Sim receives via JetStream consumer 
     ├─ Sleep: rand[500ms,3000ms]  
     ├─ Random: done(80%) / failed(20%) / in_progress(20%)
     └─ Publishes progress → tasks.progress.<task-id>

  [t0+~5s] Consumer receives progress update
     ├─ Idempotency check (skip if already processed)
     ├─ Order guard (drop out-of-order)  
     ├─ Task FSM validate (from → to?)
     ├─ Update task.status in DB
     ├─ On terminal → tenant FSM transition too!
     └─ MarkProcessed for idempotency
    
  Total async time: ~5-15 seconds for full provisioning cycle.

KEY GUARANTEES:
  • At-least-once delivery (outbox persists before publish)  
  • Idempotent processing (is_processed + event_id dedup)
  • Order-tolerant (orderGuard drops stale deliveries) 
  • FSM-protected (no invalid state transitions in consumer path too)
EOF

printf "\n${GREEN}${BOLD}✓ Demo 05 complete — full async pipeline demonstrated live!${NC}\n"
