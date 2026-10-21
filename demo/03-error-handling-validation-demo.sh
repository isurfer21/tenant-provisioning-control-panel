#!/usr/bin/env bash
# ===================================================================
# 03-error-handling-validation-demo.sh
# Demonstrates all error paths: validation, optimistic lock, FSM violations.
# Payloads are verified against internal/tenant_integration_test.go.
# ────────────────────────────────────────────────────────────────────
set -euo pipefail

API_URL="http://localhost:8080"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BOLD="\033[1m"; GREEN="\033[0;32m"; RED="\033[0;31m"
CYAN="\033[0;36m"; YELLOW="\033[0;33m"; GRAY="\033[0;90m"; MAGENTA="\033[0;35m"; NC="\033[0m"

h() { printf "\n${BOLD}═══════════${NC}\n  ${BOLD}%s${NC}\n${BOLD}═══════════\n\n" "$1"; }
sub() { printf "  ${MAGENTA}[%s]${NC} " "$1"; }
ok()  { printf "\n  ${GREEN}✓ %s${NC}" "$1"; }
fail(){ printf "\n  ${RED}✗ %s${NC}" "$1"; }
info(){ printf "\n${GRAY}%b${NC}" "$1"; }

echo ""
printf "${BOLD}══════════════════════════════════════${NC}\n"
printf " ${BOLD}Demo 03: Error Handling & Validation${NC}\n"
printf "${BOLD}══════════════════════════════════════${NC}\n\n"

# ━━━━━━━━━━━ SEED A TENANT FOR LATER DEMOS ┃━━━━━━━━━━━━━
h "SEED — Creating a clean tenant for error-path demos"
seed_resp=$(curl -s "$API_URL/v1/tenants?status=active" 2>/dev/null | python3 -c "import sys,json;d=json.load(sys.stdin);print(json.dumps(d))" || true)
count_active=$(echo "$seed_resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('data',[])))" 2>/dev/null || echo "0")

if [ "$count_active" = "0" ]; then
  seed_resp=$(curl -s -X POST "$API_URL/v1/tenants" \
    -H 'Content-Type: application/json' \
    -d '{"slug":"err-demo-seed","name":"Error Demo Tenant"}')
  ok "Created fresh tenant for demos"
else
  ok "Using existing active tenant (one already seeded)"
fi

# Get a real tenant ID to use in error demos
SEED_ID=$(curl -sf "$API_URL/v1/tenants?page=1&limit=1"2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin)['data'][0]['id'])")
SEED_VER=$(curl -sf "$API_URL/v1/tenants/$SEED_ID"2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['version'])")
info "\n  Seed tenant: id=$SEED_ID, version=$SEED_VER\n"

# ━━━━━━━━━━━ A. CREATE VALIDATION ERRORS ┃━━━━━━━━━━━━━━━
h "SECTION A — POST /v1/tenants Validation Errors"

sub "- empty slug"; ok "$(curl -s -w "\n%{http_code}" \
  -X POST "$API_URL/v1/tenants" \
  -H 'Content-Type: application/json' \
  -d '{"slug":"","name":"Has Name Field"}' 2>/dev/null | tail -1)"

sub "- uppercase slug"; ok "$(curl -s -w "\n%{http_code}" \
  -X POST "$API_URL/v1/tenants" \
  -H 'Content-Type: application/json' \
  -d '{"slug":"UPPERCASE","name":"Fail"}' 2>/dev/null | tail -1)"

sub "- leading hyph"; ok "$(curl -s -w "\n%{http_code}" \
  -X POST "$API_URL/v1/tenants" \
  -H 'Content-Type: application/json' \
  -d '{"slug":"-bad-slug","name":"Fail"}' 2>/dev/null | tail -1)"

sub "- dots in slug"; ok "$(curl -s -w "\n%{http_code}" \
  -X POST "$API_URL/v1/tenants" \
  -H 'Content-Type: application/json' \
  -d '{"slug":"has.dots","name":"Fail"}' 2>/dev/null | tail -1)"

sub "- missing name"; ok "$(curl -s -w "\n%{http_code}" \
  -X POST "$API_URL/v1/tenants" \
  -H 'Content-Type: application/json' \
  -d '{"slug":"m-no-name"}' 2>/dev/null | tail -1)"

sub "- empty body"; ok "$(curl -s -w "\n%{http_code}" \
  -X POST "$API_URL/v1/tenants" \
  -H 'Content-Type: application/json' \
  -d '' 2>/dev/null | tail -1)"

cat <<'EOF'

  All errors map to HTTP 400 Bad Request via Gin's ShouldBindJSON
  and Tenant.Validate() → services.ErrValidation.

  Slug pattern enforced: ^[a-z][-a-z0-9]{1,26}[a-z0-9]$
    • Starts with lowercase letter
    • Middle chars: hyphens, digits, or lowercase letters (1–26)  
    • Ends with alphanum (never a hyphen)
  Name: required by Gin binding + Tenant.Validate() not empty.
EOF

# ━━━━━━━━━━━ B. GET ERRORS (Not Found) ┃━━━━━━━━━━━━━━━━
h "SECTION B — GET /v1/tenants/:id Not Found Errors"

sub "- non-existent UUID"; ok "$(curl -s -w "\n%{http_code}" \
  -X GET "$API_URL/v1/tenants/nonexistent-tenant-id-does-not-exist"2>/dev/null | tail -1)"

sub "- garbage string ID"; ok "$(curl -s -w "\n%{http_code}" \
  -X GET "$API_URL/v1/tenants/not-a-real-id-here" 2>/dev/null | tail -1)"

cat <<'EOF'

  Service repo.GetByID() returns ErrNotFound → error.MapError
  maps to HTTP 404 { error_code: 404, error_message: "resource not found" }.
EOF

# ━━━━━━━━━━━ C. OPTIMISTIC LOCK CONFLICT ┃━━━━━━━━━━━━━━━
h "SECTION C — PATCH /v1/tenants/:id Optimistic Lock Conflict"

msg="Logic: service.UpdateTenant reads current row → if request.version != 0 AND \
        DB version != request version, returns ErrConflict → HTTP 409.\n\
\n\
If you omit 'version' from the payload entirely, NO lock check fires — update always succeeds.\n\
Only when you explicitly include a non-zero 'version' does the concurrency guard activate."
info "$msg"

sub "- stale explicit version (999 vs current $SEED_VER)"; ok "$(curl -s -w "\n%{http_code}" \
  -X PATCH "$API_URL/v1/tenants/$SEED_ID" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Stale attempt\",\"version\":999}" 2>/dev/null | tail -1)"

# ━━━━━━━━━━━ D. FSM VIOLATION ERRORS ┃━━━━━━━━━━━━━━━━━━━
h "SECTION D — PATCH /v1/tenants/:id FSM State-Machine Violations"

cur_ver=$(curl -sf "$API_URL/v1/tenants/$SEED_ID" 2>/dev/null | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['version'])")
cur_status=$(curl -sf "$API_URL/v1/tenants/$SEED_ID" 2>/dev/null | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['status'])")

info "Current tenant state: status=$cur_status, version=$cur_ver\n"

# Show valid transitions from current state from FSM
cat <<'EOF'
  Valid transitions (from lifecycle/tenant_fsm.go):
    provisioning → active, failed, destroying
    active       → updating, destroying
    updating     → active, failed
    destroying   → destroyed, failed
    failed       → destroying
    
  Any other transition = ErrInvalidTransition → HTTP 422
EOF

sub "- transition to unknown status"; 
curl -s -X PATCH "$API_URL/v1/tenants/$SEED_ID" \
  -H 'Content-Type: application/json' \
  -d "{\"status\":\"nonexistent-status-xyz\",\"version\":0}" 2>/dev/null | python3 -m json.tool || true

# Try an invalid transition from the current state (from a state that can't go to anything else)
case "$cur_status" in
  provisioning) target="destroyed";;
  *active*)     target="provisioning";;
  updating)     target="pending";;
  destroying)   target="active";;
  failed)       target="updating";;
  *)            target="provisioning";;
esac

sub "- transition from '$cur_status' → '$target' (invalid per FSM)"  
curl -s -X PATCH "$API_URL/v1/tenants/$SEED_ID" \
  -H 'Content-Type: application/json' \
  -d "{\"status\":\"$target\"}" 2>/dev/null | python3 -m json.tool || true

# ━━━━━━━━━━━ E. DELETE ERROR PATHS ┃━━━━━━━━━━━━━━━━━━━━
h "SECTION E — DELETE /v1/tenants/:id Error Paths"

sub "- destroy non-existent tenant"; 
curl -s -w "\n%{http_code}" \
  -X DELETE "$API_URL/v1/tenants/nonexistent-tenant-id" 2>/dev/null | tail -1

cat <<'EOF'
  
  Delete flow: GetByID → StartDestroy → validates status ∈ {active, failed, provisioning}
  If invalid: ErrInvalidTransition returned.

EOF

# ━━━━━━━━━━━ SUMMARY ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
h "SUMMARY — Error Map Reference Table"
cat <<'EOF'
HTTP 400  | Bad Request        → slug/name missing, bad format (regex)
HTTP 404  | Not Found           → tenant by ID doesn't exist  
HTTP 409  | Conflict            → duplicate slug OR optimistic lock (version mismatch)
HTTP 422  | Unprocessable Entity→ FSM transition not allowed from current state
  
Error handling layers:
  1. Handler binding    → Gin's ShouldBindJSON (must-have fields, types)
  2. Service validate   → Tenant.Validate() + store.ErrValidation  
  3. Optimistic lock    → explicit version field mismatch in UpdateTenant()
  4. FSM guard          → lifecycle.ValidateTenantTransition() blocks invalid jumps
  5. Error mapper       → MapError translates Go errors to HTTP status codes

Key fix for Update API (GAP 3):
  • Omit 'version' = unconditional update (service reads row first)
  • Include non-zero 'version' = strict optimistic concurrency control
EOF

printf "\n${GREEN}${BOLD}\n  ✓ Demo 03 complete! All error paths demonstrated.${NC}\n"
