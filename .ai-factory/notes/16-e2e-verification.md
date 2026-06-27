# 16 — E2E verify

**Task:** ROADMAP → Phase 9 — End-to-end verification → "E2E verify"
**Depth:** medium

## Goal

A repeatable end-to-end check that exercises the proxy's full write path against a running stack (proxy + Loki, plus Grafana for the one admin mint call). It proves the four behaviours that matter together: a minted token writes and the line lands in Loki, a bogus token is rejected `401`, and deactivating a token makes the very next write fail `401` with no restart. The deliverable is a `verify.sh` script (with the same steps documented for a human to run by hand), eyeballable by status code and grep, not an assertion framework.

## Design

- A single `verify.sh` at the repo root (alongside the Makefile), `bash` + `curl` only — no extra runtime, matching the "single static binary, native run" posture. It runs the four steps in order, prints each step's outcome, and exits non-zero on the first failure so it doubles as a smoke gate.
- **Endpoints come from the canon defaults, overridable by env** so the script matches the local stack without editing:
  - `PROXY_URL` (default `http://localhost:4318`) — the proxy listen address from config.
  - `LOKI_URL` (default `http://localhost:3100`) — Loki, queried directly for the read-back (the proxy has no read path).
  - `GRAFANA_TOKEN` (no default; **required**) — the operator's Grafana token used only for the admin mint call in step 1.
  - Grafana base URL itself is not needed by the script: the proxy holds it (default `http://localhost:3030`) and validates the admin call internally; the script only talks to the proxy and Loki.
- **Obtaining `GRAFANA_TOKEN`** (state it in the script header and docs): create a service-account token in Grafana (Administration → Service accounts → Add token) or a legacy API key, and export it as `GRAFANA_TOKEN`. Any valid Grafana token works — the proxy applies no role gate (canon: any valid Grafana token is accepted). This is the same credential the operator pastes into the GUI's single field.
- **Step 1 — mint a write token (admin plane):**
  - `POST $PROXY_URL/api/tokens` with header `Authorization: Bearer $GRAFANA_TOKEN` and body `{"name":"e2e-verify"}`.
  - Expect `201` (creation). Capture the response JSON; extract the plaintext `token` (the `otlp_`-prefixed value) and the `id` — both are returned by the create operation. Parse with a tiny `grep`/`sed` extraction or `jq` if present; keep the dependency optional. Save them in shell vars `WRITE_TOKEN` and `TOKEN_ID`.
  - The `name` is display-only; it carries no auth meaning and is not matched anywhere — `e2e-verify` is just so the operator can spot and clean up the row later.
- **Step 2 — write through the proxy and confirm it lands:**
  - `POST $PROXY_URL/v1/logs` with `Authorization: Bearer $WRITE_TOKEN`, `Content-Type: application/json`, and a minimal but **valid OTLP/HTTP log payload** (a single `resourceLogs` → `scopeLogs` → one `logRecord`). Embed a unique marker string in the log body and a recognisable resource attribute so the read-back query can find exactly this line. Expect a **2xx** (Loki's status relayed verbatim through the label-blind forwarder).
  - Carry the OTLP resource attributes the SDKs use so the line is indexed under the frozen labels (e.g. `project`, `service_name`, `level`) — the proxy does not add labels, so the payload must.
  - Read-back: after a short settle pause, `GET $LOKI_URL/loki/api/v1/query_range` with a LogQL selector matching a label the payload set (e.g. `{service_name="e2e-verify"}` or the project label) plus a line filter on the unique marker, and a `start`/`end` window spanning the last minute. Expect `200` and a non-empty `data.result` containing the marker. Empty result ⇒ fail.
- **Step 3 — negative, bogus token:**
  - `POST $PROXY_URL/v1/logs` with `Authorization: Bearer not-a-real-token` and any small body. Expect **`401`** and **no forward** to Loki. Same opaque `401` the proxy returns for any unknown/inactive/malformed credential.
- **Step 4 — deactivate then re-write:**
  - `PATCH $PROXY_URL/api/tokens/$TOKEN_ID` with `Authorization: Bearer $GRAFANA_TOKEN` and body `{"active":false}`. Expect a 2xx (the live toggle; no save step, no restart).
  - Immediately repeat the step-2 write with the **same** `WRITE_TOKEN`. Expect **`401`** — proving deactivation is effective on the very next write, with the per-request store lookup filtering on `active`. No restart, no reload between the toggle and the failed write.
- **Output for eyeballing:** each step prints a `PASS`/`FAIL` line with the actual status code and (for the Loki read-back) whether the marker was found. A trailing summary states overall pass/fail. The "Expected outputs/status codes" table below is reproduced in the script header so a human or agent can compare at a glance.

| Step | Call | Expected |
|---|---|---|
| 1 mint | `POST /api/tokens` (+ Grafana Bearer) | `201`, JSON with `token` (`otlp_…`) and `id` |
| 2 write | `POST /v1/logs` (+ minted Bearer) | `2xx`; then `query_range` `200` with marker present |
| 3 bogus | `POST /v1/logs` (+ bad Bearer) | `401` |
| 4 deactivate + rewrite | `PATCH /api/tokens/<id>` `{active:false}`, then `POST /v1/logs` | toggle `2xx`; rewrite `401` |

## Edge cases / watch

- **Grafana token is mandatory and step-1-only** — fail fast with a clear message if `GRAFANA_TOKEN` is unset, naming how to obtain it. It is used solely for the two admin calls (mint, deactivate); it must never be sent on `/v1/logs`, and the write token must never be sent to `/api/tokens` — that would blur the two planes the whole design keeps apart.
- **Loki ingest is not synchronous** — the write returns 2xx before the line is queryable. Use a short bounded retry/poll on the read-back (a few attempts with a small sleep) rather than a single shot, or a flaky pass on a healthy stack. Do not loosen the marker match to compensate.
- **`query_range` window and labels** — Loki rejects or empties out a query whose `start`/`end` don't bracket the entry; compute a window around "now" with margin. The selector must match a label the payload actually set, since the proxy adds none — a query on a label absent from the payload returns empty and reads as a false failure.
- **Don't log the secrets** — the script must not echo `GRAFANA_TOKEN` or the minted `WRITE_TOKEN` to stdout; print only outcomes and status codes, mirroring the proxy's own never-log-token rule.
- **Cleanup leaves a deactivated row** — step 4 deactivates the verify token (tokens are never deleted by design), so reruns accumulate inactive `e2e-verify` rows. That is acceptable; optionally mint with a timestamped name to keep them distinguishable.
- **2xx vs exact 200** — assert the write on a 2xx range, not a literal `200`, because Loki's OTLP accept code is relayed verbatim and may be `204`/`200` depending on version; the proxy does not normalise it.

## Out of scope

- Driving a real SDK binary — this script substitutes a hand-built OTLP payload for an SDK call; exercising an actual `observe-*` client end-to-end is a separate effort. The ROADMAP item's "a real SDK writes" is satisfied at the protocol level here (same endpoint, same Bearer header, same OTLP body shape).
- Standing up the stack — the script assumes proxy, Loki, and Grafana are already running (via `make backend-up` / the local run path); it does not start or health-gate them beyond optionally hitting `/healthz` first.
- Testing the GUI, admin list endpoint, reactivation, or the `5xx`-on-upstream-failure path — those are covered by their own tasks; this verify focuses on the four ROADMAP-named outcomes.
- Any CI integration or assertion framework — plain `curl` + exit codes only.

## Done when

- Running `verify.sh` against the local stack (with a valid `GRAFANA_TOKEN` exported) completes all four steps and reports overall PASS.
- Step 2: the minted token writes and the unique marker is found by the `query_range` read-back in Loki.
- Step 3: the bogus token is rejected with `401`.
- Step 4: after `PATCH …{active:false}`, the next write with that same token fails with `401`, with no restart or reload between the toggle and the failed write.
- Each step's status code and pass/fail is visible in the output, and no token value is printed.
