# Plan Review: 16-e2e-verify

**Plan:** `.ai-factory/plans/16-e2e-verify.md`
**Files Reviewed:** plan + `cmd/proxy/main.go`, `internal/admin/tokens.go`, `internal/admin/auth.go`, `internal/proxy/auth.go`, `internal/proxy/handler.go`, `internal/store/token.go`, `internal/config/config.go`, `internal/grafana/client.go`, `README.md`, `../observe-contract/golden-record.json`, `../backend/loki/loki.yaml`, `../backend/verify.sh`
**Risk Level:** 🟡 Medium

## Verified correct against the codebase

The plan's API assumptions all check out against the actual implementation:

- **Endpoints & ports.** `PROXY_URL` default `http://localhost:4318` matches `config.defaultListen = ":4318"`; `LOKI_URL` default `http://localhost:3100` matches `defaultLoki`. Routes `/healthz`, `/v1/logs`, `/api/tokens`, `/api/tokens/` all exist in `newMux` (`main.go:110-113`).
- **Mint (Task 2).** `POST /api/tokens` with `{"name":...}` returns `201` (`tokens.go:98`, `http.StatusCreated`). Response JSON carries `id`, `token`, `name`, `active`, `created_at` — so extracting `token` and `id` is valid, and the token value is `otlp_`-prefixed (`store/token.go:43`).
- **Write auth (Steps 2/3).** Bearer check happens before the body is ever read (`proxy/auth.go:40-60`); an unknown/missing/malformed token → `401`, a valid active token → forward. So the "bogus token → 401" and "deactivated token → 401 on next write, same body" assertions are sound, and body content is irrelevant to the 401 path.
- **Write status relay (Step 2).** `handler.go:47` relays Loki's status verbatim (Loki OTLP answers `204`), so asserting a **2xx range** rather than literal `200` is the right call.
- **Deactivation (Step 4).** `PATCH /api/tokens/<id>` with `{"active":false}` returns `200` (`tokens.go:134`); `SetActive` flips the flag in SQL and `Lookup` filters `active=1` in SQL with no cache (`store/token.go:101-119`), so "next write fails, no restart" is genuinely what the code does. `DELETE` returns `204` (`tokens.go:145`).
- **Label selector.** `../backend/loki/loki.yaml` promotes `project`, `service_name`, `level` to index labels (resource `service.name` → label `service_name`). The query `{service_name="e2e-verify"}` is a valid stream selector. `auth_enabled: false` in loki.yaml confirms **no `X-Scope-OrgID` header is needed** for the write or the query — good, the plan correctly omits it.
- **OTLP shape.** `golden-record.json` matches the described `resourceLogs → scopeLogs → logRecords` structure the plan tells the implementer to follow.

## Critical Issues

**1. The payload must set `timeUnixNano`/`observedTimeUnixNano` to *now*, or the readback will always FAIL.**
Task 3 tells the implementer to build the record "following the … shape from `golden-record.json`" but never says to set the timestamp. `golden-record.json` hardcodes `"timeUnixNano": "1718632800000000000"` (June 2024). If that value is copied through, the log lands in Loki dated 2024 — while the readback query window is `start = now-120s` … `end = now+10s` (Task 3). The record is outside the window, `data.result` is empty, and the script reports a false **FAIL** even though ingestion succeeded.
The existing `../backend/verify.sh` sidesteps this by querying `start=1700000000000000000` (a fixed 2023 floor) *and* calling `POST $LOKI/flush` + `sleep 3`, precisely because its fixtures use historical timestamps.
**Fix:** state explicitly in Task 3 that `timeUnixNano` (and `observedTimeUnixNano`) must be set to the current time in nanoseconds — e.g. `TS_NS="$(date +%s)000000000"` — so the record falls inside the `now±` window. This is the one change that determines whether the happy-path step can ever pass.

## Recommendations (non-blocking)

**2. Note that no `/flush` is required — but say why.**
With a *current* timestamp (per fix #1) the record is queryable straight from the ingester head chunk, so the 5×2s poll (10s) is adequate and no flush is needed. Worth one sentence in Task 3 so the implementer doesn't cargo-cult the `flush`+`sleep` from `../backend/verify.sh` (which only exists there because those fixtures are historical and get cut into cold chunks immediately).

**3. Acknowledge the pre-existing `../backend/verify.sh` and its scope difference.**
The root backend repo already ships `../backend/verify.sh` (`make backend-verify`) that ingests `golden-record.json` **straight into Loki** and queries it back. The new script is justified — it exercises the **proxy write path and its auth planes**, which the backend script bypasses entirely — but the plan should say so, so the two aren't seen as redundant. The new script could also reuse that script's `pass`/`fail` helper idiom and query-window pattern for consistency.

**4. Clarify which Grafana token type Validate accepts.**
`grafana/client.go:91-112` validates by calling `GET /api/user` with the Bearer token; only a `200` counts as valid. Task 1's "fail fast … naming how to obtain the token" guidance should be specific: `/api/user` authenticates a **Grafana user/session-style token**; a bare service-account token may not resolve a user on all Grafana versions. Naming the expected token type in the preflight error will save the operator a confusing `401` on the mint step (which would otherwise look like a proxy bug rather than a wrong-token-type issue).

## Positive Notes

- Guard rails are correct and well-placed: "never echo `GRAFANA_TOKEN` or minted token" aligns with the code's own no-log-the-token discipline; "send the Grafana token only to `/api/tokens`, the write token only to `/v1/logs`" correctly mirrors the two-plane split that is this service's whole reason for existing.
- Asserting a 2xx *range* (not literal `200`) for the write, and expecting `2xx` for the PATCH, correctly anticipate Loki's `204` and the handler's `200` respectively — a common place plans get the exact code wrong.
- Testing deactivation by re-sending the *same* token+body and expecting `401` is exactly the behaviour the store's SQL-level `active=1` filter guarantees; the plan tests the real invariant, not a proxy for it.
- Env-overridable endpoints with canon defaults matching `config.go`, plus optional `/healthz` preflight, make the script a usable smoke gate.

## Context Gates

- **Architecture:** WARN (advisory) — `.ai-factory/ARCHITECTURE.md` not present in this sub-repo; the plan respects the two-plane boundary and the label-blind guarantee described in `CLAUDE.md`, so no boundary violation.
- **Rules:** No `.ai-factory/RULES.md` present — nothing to enforce.
- **Roadmap:** Plan is a numbered `.ai-factory/plans/` artifact consistent with the sub-repo's roadmap flow; scoped correctly to the proxy repo (not the root, per the routing rules), since it verifies the proxy's own write path.

---

Overall the plan is accurate about the codebase and architecturally sound. Issue #1 is a real correctness bug that will make the central happy-path step fail on every run unless the timestamp is set to now; address it before implementation.
