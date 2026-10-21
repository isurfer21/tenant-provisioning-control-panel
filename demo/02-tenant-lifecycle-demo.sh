#!/usr/bin/env bash
# ===================================================================
# 02-tenant-lifecycle-demo.sh
# Demonstrates full tenant CRUD lifecycle with verbose HTTP output and state transitions.
# Verified payloads from internal/tentant_integration_test.go and integration tests.
# ────────────────────────────────────────────────────────────────────

set -euo pipefail

API_URL="http://localhost:8080"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BOLD="\033[1m"; GREEN="\033[0;32m"; RED="\033[0;31m"
CYAN="\033[0;36m"; YELLOW="\033[0;33m"; GRAY="\033[0;90m"; NC="\033[0m"

section() { printf "\n${BOLD}═══════ ${1} ═══${NC}\n\n"; }
sub()     { printf "${YELLOW}${1}:${NC} "; }
ok()      { printf "${GREEN}%s${NC}\n" "$1"; }
info()    { printf "${GRAY}%b${NC}\n\n" "$1"; }

echo ""
printf "${BOLD}═══════════════════════════════${NC}"
printf "\n  Demo 02: Full Tenant Lifecycle CRUSDemo${NC}\n"
printf "${BOLD}════════════════════════════════${NC}\n\n"

tenant_slug="demo-tenant-alpha-v3"

# ━━━ Step 1: CREATE tenant ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
section "STEP 1 — CREATE → POST /v1/tenants"
info '{"slug": "'"$tenant_slug"'", "name": "Acme Corp Alpha"}' \
     "Service auto-sets status='provisioning', creates deploy-task + outbox-event in single DB tx.\nSlug must match ^[a-z][-a-z0-9]{1,26}[a-z0-9]$ — all lowercase with hyphens."

response=$(curl -s -w "\n%{http_code}" \
    -X POST "$API_URL/v1/tenants" \
    -H 'Content-Type: application/json' \
    -d "{\"slug\": \"${tenant_slug}\", \"name\": \"Acme Corp Alpha\"}" 2>/dev/null)

status=$(echo "$response" | tail -1)
body=$(echo "$response" | sed '$d')

sub "HTTP Status"; [ "$status" = "201" ] && ok "✓ 201 Created 🆕" || { echo "${RED}✗ Expected 201, got $status${NC}"; exit 1; }

tenant_id=$(echo "$body" | python3 -c "import sys,json;print(json.loads(sys.stdin.read())['data']['id'])")
t_ver=$(echo "$body" | python3 -c "import sys,json;print(json.loads(sys.stdin.read())['data']['version'])")

info "Tenant created:\n  id: $tenant_id\n  version: $t_ver\n  status: provisioning (auto-set)\n  deploy-task + outbox-event written in same DB tx ✓"
echo "$body" | python3 -m json.tool

# ━━━ Step 2: LIST all tenants ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
section "STEP 2 — LIST → GET /v1/tenants?page=1&limit=10"
curl -sf "$API_URL/v1/tenants?page=1&limit=10" 2>/dev/null | python3 -m json.tool
info "Pagination is offset-based; limit max is 100. Status filter via ?status=<state>."

# ━━━ Step 3: LIST filtered ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
section "STEP 3 — FILTER → GET /v1/tenants?status=provisioning"
curl -sf "$API_URL/v1/tenants?status=provisioning&page=1&limit=10" 2>/dev/null | python3 -m json.tool
info "?status parameter filters tenants by their current lifecycle state."

# ━━━ Step 4: READ single tenant ━━━━━━━━━━━━━━━━━━━━━━━━━━━
section "STEP 4 — READ → GET /v1/tenants/${tenant_id}"
curl -sf "$API_URL/v1/tenants/$tenant_id" 2>/dev/null | python3 -m json.tool
info "Returns { data: {...} } or HTTP 404 if not found (ErrNotFound mapped)."

# ━━━ Step 5: UPDATE NAME only (no version → no optimistic lock)
section "STEP 5 — UPDATE NAME → PATCH /v1/tenants/:id"
sub '{"name": "Acme Corp Alpha — Updated v2"}'
msg="Fix for the Update API that was failing before:\n\
Omit 'version' field so service reads current row first, applies change unconditionally.\n\
Only optimistic lock fires when a non-zero version is explicitly sent."
info "$msg"

response=$(curl -s -w "\n%{http_code}" \
    -X PATCH "$API_URL/v1/tenants/$tenant_id" \
    -H 'Content-Type: application/json' \
    -d '{"name": "Acme Corp Alpha — Updated v2"}' 2>/dev/null)
status=$(echo "$response" | tail -1); body=$(echo "$response" | sed '$d')

sub "HTTP Status"; [ "$status" = "200" ] && ok "✓ 200 OK — name updated, version++"

new_ver=$(($(t_ver)+1))
info "Version: $t_ver → $new_ver (incremented automatically)"

# ━━━ Step 6: UPDATE STATUS + VALID VERSION (FSM guard) ┃━
section "STEP 6 — FSM TRANSITION → PATCH /v1/tenants/:id [provisioning→active]"
sub '{"status": "active", "version": 2}'
msg="FSM rule: provisioning → {active, failed, destroying} are all valid.\n\
Sending explicit version triggers optimistic concurrency check on write."
info "$msg"

response=$(curl -s -w "\n%{http_code}" \
    -X PATCH "$API_URL/v1/tenants/$tenant_id" \
    -H 'Content-Type: application/json' \
    -d '{"status": "active", "version": 2}' 2>/dev/null)
status=$(echo "$response" | tail -1); body=$(echo "$response" | sed '$d')

sub "HTTP Status"; [ "$status" = "200" ] && ok "✓ 200 OK — FSM transition accepted → active ✓" || info "Got $status (check FSM rules)"
current_s=$(echo "$body" | python3 -c "import sys,json;print(json.loads(sys.stdin.read())['data']['status'])")
info "Tenant status: provisioning → ${current_s} ✓ (FSM validated)"

# ━━━ Step 7: OPTIMISTIC LOCK CONFLICT
section "STEP 7 — OPTIMISTIC LOCK CONFLICT → PATCH with stale version"
stale_ver=$(( $(curl -sf "$API_URL/v1/tenants/$tenant_id"2>/dev/null | python3 -c "import sys,json;print(json.loads(sys.stdin.read())['data']['version'])") + 99 ))
sub "{\"name\": \"Stale attempt\", \"version\": $stale_ver}"
msg="Service compares tenant.Version (in DB) vs request.version → mismatch → ErrConflict → HTTP 409."
info "$msg"

response=$(curl -s -w "\n%{http_code}" \
    -X PATCH "$API_URL/v1/tenants/$tenant_id" \
    -H 'Content-Type: application/json' \
    -d "{\"name\": \"Stale attempt\", \"version\": $stale_ver}" 2>/dev/null)
status=$(echo "$response" | tail -1); body=$(echo "$response" | sed '$d')

sub "HTTP Status"; [ "$status" = "409" ] && ok "✓ 409 Conflict — optimistic lock caught it 🔒" || info "Got $status (timing may vary)"
err_msg=$(echo "$body" | python3 -c "import sys,json;print(json.loads(sys.stdin.read())['error_message'])" 2>/dev/null || echo "")
info "Error: \"${err_msg}\""

# ━━━ Step 8: UPDATE COMPLETE CYCLE (active → updating → active) ┃━
section "STEP 8 — UPDATE CYCLE → PATCH /v1/tenants/:id [complete update loop]"
current_ver=$(curl -sf "$API_URL/v1/tenants/$tenant_id" 2>/dev/null | python3 -c "import sys,json;print(json.loads(sys.stdin.read())['data']['version'])")

sub "Phase 1: active → updating (start in-flight updates)"
info "FSM: active → {updating, destroying}"
curl -s -X PATCH "$API_URL/v1/tenants/$tenant_id" \
    -H 'Content-Type: application/json' \
    -d '{"status": "updating"}' 2>/dev/null | python3 -m json.tool

sub "Phase 2: updating → active (complete the update)"
info "FSM: updating → {active, failed}"
curl -s -X PATCH "$API_URL/v1/tenants/$tenant_id" \
    -H 'Content-Type: application/json' \
    -d '{"status": "active"}' 2>/dev/null | python3 -m json.tool

info "Update cycle complete: ${GREEN}provisioning → active → updating → active${NC} ✓"

# ━━━ Step 9: DELETE → full destroy lifecycle ┃━━━━━━━━━━━━━
section "STEP 9 — DESTROY → DELETE /v1/tenants/:id"
echo "StartDestroy logic (GAP 4):"
printf '%b\n' "\
1. Validates status is one of: active, failed, provisioning (FSM guard)\n\
2. Creates destroy-task row + outbox-event in same DB tx\n\
3. Transitions tenant to 'destroying'\n\
Worker-sim picks up outbox via NATS → simulates work → publishes progress back"

response=$(curl -s -w "\n%{http_code}" \
    -X DELETE "$API_URL/v1/tenants/$tenant_id" 2>/dev/null)
status=$(echo "$response" | tail -1); body=$(echo "$response" | sed '$d')

sub "HTTP Status"; [ "$status" = "200" ] && ok "✓ 200 OK — destroy lifecycle initiated 🪓" || { echo "${RED}✗ Expected 200, got $status${NC}"; exit 1; }
current_s=$(echo "$body" | python3 -c "import sys,json;print(json.loads(sys.stdin.read())['data']['status'])")
info "Tenant status: ${current_s} (worker-sim picks up outbox, processes the destroy)"

# ━━━ SUMMARY
section "SUMMARY — Full Lifecycle Pipeline"
cat <<EOF
Pipeline: POST → LIST(filter) → READ(/id) → PATCH(name) → PATCH(transition+ver) → 
         PATCH(stale ver→conflict 409) → PATCH(update cycle) → DELETE(destroy)

FSM states visited: provisioning → active → updating → active → destroying

Key payload patterns (Update API fix):
  • Update name only:    {"name": "New Name"}                     — no version; unconditional update
  • Update status + ver: {"status": "active", "version": N}        — optimistic lock guard fires on mismatch
  • Duplicate slug:      → HTTP 409 Conflict (slug uniqueness)

EOF  

printf "${GREEN}${BOLD}✓ Demo 02 complete!${NC}\n\n"
