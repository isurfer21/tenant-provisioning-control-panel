#!/usr/bin/env bash
set -euo pipefail
BASE=http://localhost:8080
curl -s -X POST "$BASE/v1/tenants" \
    -H "Content-Type: application/json" \
    -d '{"slug":"t-fix","name":"FixTest"}' > /tmp/create.json

TID=$(python3 -c "import json; print(json.load(open('/tmp/create.json'))['data']['id'])")
echo "TID=$TID"

# DELETE raw
curl -v -X DELETE "$BASE/v1/tenants/$TID" 2>&1

# PATCH with version
curl -s -X PATCH "$BASE/v1/tenants/$TID" \
    -H "Content-Type: application/json" \
    -d '{"name":"Updated","version":1}' 2>&1
