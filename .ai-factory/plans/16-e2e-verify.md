# Plan: E2E verify

## Context
Add a repeatable `verify.sh` at the repo root that exercises the proxy's full write path against a running proxy + Loki (+ Grafana for the one admin mint), proving the four behaviours that matter together: minted token writes and lands in Loki, bogus token → `401`, and deactivation takes effect on the very next write → `401`, no restart.

This is distinct from the root repo's pre-existing `../backend/verify.sh` (`make backend-verify`), which ingests `golden-record.json` **straight into Loki** and queries it back — bypassing the proxy entirely. This new script instead exercises the **proxy write path and its two auth planes** (write-token auth on `/v1/logs`, Grafana-token auth on `/api/tokens`), which the backend script never touches. The two are complementary, not redundant. Where convenient, mirror the backend script's `pass`/`fail` helper idiom and query-window pattern for consistency, but do **not** copy its `POST /flush` + `sleep 3` step — that exists there only because its fixtures use historical timestamps (see Task 3).

## Settings
- Testing: no
- Logging: minimal
- Docs: yes (expected status codes in README)

## Tasks

### Phase 1: Script foundation

- [x] **Task 1: Script scaffold, env config, and preflight**
  Files: `verify.sh`
  Create `verify.sh` at the repo root (alongside `Makefile`), `#!/usr/bin/env bash` with `set -euo pipefail`, `bash` + `curl` only. Add a header comment reproducing the expected status-code table from the spec (step / call / expected). Define env-overridable endpoints with canon defaults: `PROXY_URL` (default `http://localhost:4318`), `LOKI_URL` (default `http://localhost:3100`), and required `GRAFANA_TOKEN` (no default). Fail fast with a clear message if `GRAFANA_TOKEN` is unset — the message must name the expected token type: the proxy validates via `GET /api/user`, so a **Grafana user/session-style token** is expected; a bare service-account token may not resolve a user on all Grafana versions and would surface as a confusing `401` on the mint step. State how to obtain one (Grafana → user profile / API tokens). Add a `jq`-if-present / `grep`+`sed` fallback helper for JSON field extraction (keep `jq` optional). Add small `pass`/`fail` output helpers that print a per-step `PASS`/`FAIL` line with the actual status code, and a global failure flag / exit-non-zero-on-first-failure behaviour so the script doubles as a smoke gate. Optionally probe `GET $PROXY_URL/healthz` first. Guard: never echo `GRAFANA_TOKEN` or minted token values — print only outcomes and status codes. Make the file executable (`chmod +x`).

### Phase 2: Write-path steps

- [x] **Task 2: Step 1 — mint a write token (admin plane)** (depends on Task 1)
  Files: `verify.sh`
  `POST $PROXY_URL/api/tokens` with header `Authorization: Bearer $GRAFANA_TOKEN` and body `{"name":"e2e-verify"}`. Expect `201`. Capture the response body, extract the plaintext `token` (the `otlp_`-prefixed value) and the `id` into shell vars `WRITE_TOKEN` and `TOKEN_ID` using the Task 1 extraction helper. Print `PASS`/`FAIL` with the status code (never the token). Guard: send the Grafana token here only — never on `/v1/logs`.

- [x] **Task 3: Step 2 — write OTLP through the proxy and confirm it lands** (depends on Task 2)
  Files: `verify.sh`
  Build a minimal valid OTLP/HTTP JSON log payload following the `resourceLogs` → `scopeLogs` → `logRecords` shape from `../observe-contract/golden-record.json`: resource attributes `project = "e2e"` and `service.name = "e2e-verify"`; a log record with `severityNumber = 9`, a `level = "info"` attribute, and a unique marker in `body.stringValue` (e.g. `MARKER="e2e-$(date +%s)"`). **Critical:** set `timeUnixNano` and `observedTimeUnixNano` to the current time in nanoseconds — `TS_NS="$(date +%s)000000000"` — do **not** copy the `golden-record.json` hardcoded `1718632800000000000` (June 2024) value; a 2024-dated record falls outside the `now±` readback window below and the step would falsely FAIL despite a successful write. With a current timestamp the record is queryable straight from the ingester head chunk, so the 5×2s poll is sufficient and **no `POST /flush` is needed** (unlike `../backend/verify.sh`, whose fixtures are historical and get cut into cold chunks immediately). `POST $PROXY_URL/v1/logs` with `Authorization: Bearer $WRITE_TOKEN` and `Content-Type: application/json`; assert a **2xx range** (not literal `200` — Loki's accept code is relayed verbatim). Then read back: poll `GET $LOKI_URL/loki/api/v1/query_range` up to 5 times with 2s sleep, params `query={service_name="e2e-verify"} |= "e2e-"`, `start=$(($(date +%s) - 120))000000000`, `end=$(($(date +%s) + 10))000000000` (Unix nanoseconds), `limit=10`. Non-empty `data.result` ⇒ marker found → PASS; empty after 5 attempts ⇒ FAIL. Guard: the proxy adds no labels — the payload must carry `project`, `service.name`, `level` itself.

- [x] **Task 4: Steps 3 & 4 — bogus token and deactivation, plus summary and cleanup** (depends on Task 3)
  Files: `verify.sh`
  Step 3: `POST $PROXY_URL/v1/logs` with `Authorization: Bearer not-a-real-token` and any small body; expect **`401`**. Step 4: `PATCH $PROXY_URL/api/tokens/$TOKEN_ID` with `Authorization: Bearer $GRAFANA_TOKEN` and body `{"active":false}`; expect a 2xx; then immediately repeat the Step 2 write with the same `$WRITE_TOKEN` and expect **`401`** (proving next-write deactivation, no restart/reload). Print a trailing overall PASS/FAIL summary; exit non-zero if any step failed. Optionally issue `DELETE $PROXY_URL/api/tokens/$TOKEN_ID` at the end to hard-remove the verify token (note that inactive `e2e-verify` rows otherwise accumulate on reruns).

### Phase 3: Documentation

- [x] **Task 5: Document verify.sh and expected status codes** (depends on Task 4)
  Files: `README.md`
  Add a short "End-to-end verification" section describing how to run `verify.sh` against the local stack: export `GRAFANA_TOKEN` (and optionally `PROXY_URL` / `LOKI_URL`) then run the script; it exercises the four steps and exits non-zero on the first failure. Include the expected status-code table (mint `201`; write `2xx` + `query_range` marker present; bogus `401`; deactivate `2xx` then rewrite `401`). Prose + behavior only, no file trees, matching house doc style.

## Commit Plan
- **Commit 1** (after tasks 1-4): "Add end-to-end verify script for the write path"
- **Commit 2** (after task 5): "Document end-to-end verification in README"
