#!/usr/bin/env bash
# End-to-end verification of the write path: mint a token via the admin
# plane, write OTLP through the proxy and confirm it lands in Loki, prove a
# bogus token is rejected, and prove deactivation takes effect on the very
# next write — no restart, no reload.
#
# Distinct from ../backend/verify.sh (make backend-verify), which ingests
# golden-record.json straight into Loki and bypasses the proxy entirely.
# This script instead exercises the proxy's two auth planes: write-token
# auth on /v1/logs and Grafana-token auth on /api/tokens.
#
# Requires a running proxy + Loki, and Grafana reachable for the one admin
# mint/deactivate step.
#
# Expected status codes:
#   Step 1  POST   /api/tokens        (Grafana token)  -> 201
#   Step 2  POST   /v1/logs           (write token)     -> 2xx, then marker
#                                                            found via
#                                                            query_range
#   Step 3  POST   /v1/logs           (bogus token)      -> 401
#   Step 4  PATCH  /api/tokens/<id>   (Grafana token)   -> 2xx, then the
#                                                            same write
#                                                            token rewrite
#                                                            -> 401
#
# Usage:
#   GRAFANA_TOKEN=<grafana-token> ./verify.sh
#
# Env overrides (canon defaults shown):
#   PROXY_URL      http://localhost:4318
#   LOKI_URL       http://localhost:3100
#   GRAFANA_TOKEN  required, no default — see below.
#
# GRAFANA_TOKEN accepts any valid Grafana token: the proxy applies no role
# gate, validating it via GET /api/user, where any 200 response means
# valid. A Grafana service-account token (glsa_..., created under
# Administration -> Service accounts -> Add token) is the recommended
# choice; a user API token or a session token also works.

set -euo pipefail

PROXY_URL="${PROXY_URL:-http://localhost:4318}"
LOKI_URL="${LOKI_URL:-http://localhost:3100}"

RESP_BODY="$(mktemp)"
trap 'rm -f "$RESP_BODY"' EXIT

FAIL=0

pass() { echo "  PASS  $1 (HTTP $2)"; }
# fail() prints the failing step and aborts immediately (exit non-zero on
# first failure) so this script doubles as a smoke gate, not just a report.
fail() { echo "  FAIL  $1 (HTTP $2)"; FAIL=1; exit 1; }

# json_field extracts a top-level string field from a flat JSON object.
# Prefers jq; falls back to grep+sed when jq is not installed.
json_field() {
  local json="$1" key="$2"
  if command -v jq >/dev/null 2>&1; then
    printf '%s' "$json" | jq -r --arg k "$key" '.[$k] // empty'
  else
    printf '%s' "$json" | grep -o "\"$key\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" | head -1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/'
  fi
}

if [ -z "${GRAFANA_TOKEN:-}" ]; then
  echo "GRAFANA_TOKEN is not set." >&2
  echo "" >&2
  echo "The proxy applies no role gate — any valid Grafana token works. A" >&2
  echo "Grafana service-account token (glsa_..., created under Administration ->" >&2
  echo "Service accounts -> Add token) is the recommended choice; a user API" >&2
  echo "token or a session token also works." >&2
  exit 1
fi

echo "=== observe-write-proxy e2e verify ==="
echo "Proxy: $PROXY_URL   Loki: $LOKI_URL"
echo ""

echo "Preflight: proxy /healthz"
if curl -sf --max-time 5 "$PROXY_URL/healthz" >/dev/null 2>&1; then
  pass "proxy reachable" 200
else
  fail "proxy /healthz unreachable — is the proxy running?" "n/a"
fi

echo ""
echo "Step 1: mint a write token (admin plane)"
STATUS="$(curl -sS -o "$RESP_BODY" -w '%{http_code}' -X POST "$PROXY_URL/api/tokens" \
  -H "Authorization: Bearer $GRAFANA_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"e2e-verify"}')"
BODY="$(cat "$RESP_BODY")"
if [ "$STATUS" = "201" ]; then
  pass "mint write token" "$STATUS"
else
  fail "mint write token" "$STATUS"
fi

WRITE_TOKEN="$(json_field "$BODY" token)"
TOKEN_ID="$(json_field "$BODY" id)"
if [ -z "$WRITE_TOKEN" ] || [ -z "$TOKEN_ID" ]; then
  fail "mint write token: response missing token/id" "$STATUS"
fi

echo ""
echo "Step 2: write OTLP through the proxy and confirm it lands"
MARKER="e2e-$(date +%s)"
TS_NS="$(date +%s)000000000"
PAYLOAD=$(cat <<EOF
{
  "resourceLogs": [
    {
      "resource": {
        "attributes": [
          { "key": "project", "value": { "stringValue": "e2e" } },
          { "key": "service.name", "value": { "stringValue": "e2e-verify" } }
        ]
      },
      "scopeLogs": [
        {
          "scope": { "name": "observe-write-proxy-verify", "version": "0.1.0" },
          "logRecords": [
            {
              "timeUnixNano": "$TS_NS",
              "observedTimeUnixNano": "$TS_NS",
              "severityNumber": 9,
              "severityText": "INFO",
              "body": { "stringValue": "$MARKER" },
              "attributes": [
                { "key": "level", "value": { "stringValue": "info" } }
              ]
            }
          ]
        }
      ]
    }
  ]
}
EOF
)

STATUS="$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$PROXY_URL/v1/logs" \
  -H "Authorization: Bearer $WRITE_TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary "$PAYLOAD")"
if [[ "$STATUS" =~ ^2 ]]; then
  pass "write via proxy" "$STATUS"
else
  fail "write via proxy" "$STATUS"
fi

FOUND=0
for _ in 1 2 3 4 5; do
  START="$(($(date +%s) - 120))000000000"
  END="$(($(date +%s) + 10))000000000"
  RESULT="$(curl -sS --max-time 10 -G "$LOKI_URL/loki/api/v1/query_range" \
    --data-urlencode "query={service_name=\"e2e-verify\"} |= \"$MARKER\"" \
    --data-urlencode "start=$START" \
    --data-urlencode "end=$END" \
    --data-urlencode 'limit=10' || true)"
  if echo "$RESULT" | grep -q '"values":\[\["'; then
    FOUND=1
    break
  fi
  sleep 2
done

if [ "$FOUND" = "1" ]; then
  pass "read back via query_range (marker found)" "$STATUS"
else
  fail "read back via query_range (marker not found after 5 attempts)" "$STATUS"
fi

echo ""
echo "Step 3: bogus token is rejected"
STATUS="$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$PROXY_URL/v1/logs" \
  -H 'Authorization: Bearer not-a-real-token' \
  -H 'Content-Type: application/json' \
  -d '{}')"
if [ "$STATUS" = "401" ]; then
  pass "bogus token rejected" "$STATUS"
else
  fail "bogus token rejected" "$STATUS"
fi

echo ""
echo "Step 4: deactivation takes effect on the very next write"
STATUS="$(curl -sS -o /dev/null -w '%{http_code}' -X PATCH "$PROXY_URL/api/tokens/$TOKEN_ID" \
  -H "Authorization: Bearer $GRAFANA_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"active":false}')"
if [[ "$STATUS" =~ ^2 ]]; then
  pass "deactivate write token" "$STATUS"
else
  fail "deactivate write token" "$STATUS"
fi

STATUS="$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$PROXY_URL/v1/logs" \
  -H "Authorization: Bearer $WRITE_TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary "$PAYLOAD")"
if [ "$STATUS" = "401" ]; then
  pass "write after deactivation rejected (no restart, no reload)" "$STATUS"
else
  fail "write after deactivation rejected" "$STATUS"
fi

# Best-effort cleanup: hard-remove the verify token so inactive e2e-verify
# rows don't accumulate on reruns. Not gated on success — a cleanup failure
# is not a verification failure.
curl -sS -o /dev/null -X DELETE "$PROXY_URL/api/tokens/$TOKEN_ID" \
  -H "Authorization: Bearer $GRAFANA_TOKEN" >/dev/null 2>&1 || true

echo ""
if [ "$FAIL" -eq 0 ]; then
  echo "=== all steps passed ==="
  exit 0
else
  echo "=== verification failed ==="
  exit 1
fi
