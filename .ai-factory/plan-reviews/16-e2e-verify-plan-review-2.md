# Plan Review: 16-e2e-verify (round 2)

**Plan:** `.ai-factory/plans/16-e2e-verify.md`
**Files Reviewed:** plan + `cmd/proxy/main.go`, `internal/admin/tokens.go`, `internal/admin/auth.go`, `internal/proxy/auth.go`, `internal/proxy/handler.go`, `internal/proxy/forwarder.go`, `internal/store/token.go`, `internal/config/config.go`, `internal/grafana/client.go`, `../backend/verify.sh`, round-1 review
**Risk Level:** 🟢 Low

## Round-1 findings — all resolved

The single critical issue and three recommendations from `16-e2e-verify-plan-review-1.md` are now folded into the plan text:

- **Timestamp = now (was critical).** Task 3 now states explicitly: set `timeUnixNano`/`observedTimeUnixNano` to `TS_NS="$(date +%s)000000000"` and *not* the `golden-record.json` June-2024 constant, so the record lands inside the `now±` readback window. This closes the false-FAIL that would otherwise hit every run.
- **No `/flush` — with the why.** Task 3 now explains that a current timestamp is queryable straight from the ingester head chunk, so the 5×2s poll suffices and the `POST /flush` + `sleep 3` from `../backend/verify.sh` must not be cargo-culted (those fixtures are historical).
- **Scope vs `../backend/verify.sh`.** The Context section now names the pre-existing backend script, states it bypasses the proxy, and frames the new script as complementary (proxy write path + two auth planes).
- **Grafana token type.** Task 1 now requires the preflight error to name a Grafana user/session-style token and note that a bare service-account token may not resolve a user, surfacing as a confusing `401` on mint.

## Re-verified against the current code

- **Ports/endpoints.** `PROXY_URL` default `:4318` = `config.defaultListen`; `LOKI_URL` default `http://localhost:3100` = `defaultLoki`. `/healthz`, `/v1/logs`, `/api/tokens`, `/api/tokens/` all registered in `newMux` (`main.go:110-114`).
- **Mint → 201.** `tokens.go:98` returns `http.StatusCreated`; response carries `id`, `token` (`otlp_`-prefixed, `store/token.go:43`), `name`, `active`, `created_at`. Extracting `WRITE_TOKEN`/`TOKEN_ID` is valid.
- **Duplicate mint name is safe.** `Create` (`token.go:48-67`) has no unique constraint on `name`; `id`/`token` are random. Reruns with the fixed `e2e-verify` name insert fresh rows and never 500 — so capturing a per-run `TOKEN_ID` is correct, and the plan's note about inactive rows accumulating (with optional `DELETE` cleanup) is accurate.
- **Write status relay is 2xx-range.** `handler.go:47` relays Loki's status verbatim (Loki OTLP → `204`); asserting a 2xx range, not literal `200`, is right.
- **Bogus/deactivated → 401.** `proxy/auth.go:40-58` checks the Bearer token before the body is read; unknown *and* inactive collapse to the same `401` (`store/token.go:101-119`, SQL `active=1`, no cache). "Same token+body → 401 on next write, no restart" is exactly the invariant.
- **Deactivate → 200.** `PATCH /api/tokens/<id>` with `{"active":false}` returns `http.StatusOK` (`tokens.go:134`); `DELETE` returns `204` (`tokens.go:145`).
- **Grafana validation.** `grafana/client.go:91-112` validates via `GET /api/user`; only `200` is valid — matches Task 1's token-type guidance.

## Recommendations (non-blocking)

1. **URL-encode the `query_range` params.** The `query` value (`{service_name="e2e-verify"} |= "e2e-"`) contains braces, quotes, spaces and `|=` — it must be sent with `curl -G --data-urlencode`, exactly as `../backend/verify.sh:53-57` does. The plan says to "mirror the backend script's … query-window pattern", which implicitly covers this, but calling out `-G --data-urlencode` in Task 3 removes any chance of a naive `?query=...` interpolation that would silently return empty and read as a FAIL.

2. **Grafana-unreachable is 503, not 401.** For the operator's benefit only: the admin gate returns `401` when the Grafana token is rejected (`admin/auth.go:39`) but `503 "grafana unreachable"` when Grafana is down (`admin/auth.go:36`). Task 1's error text could optionally mention that a `503` on mint means "Grafana not reachable at `GRAFANA_URL`" vs `401` meaning "wrong/invalid token" — saves a diagnosis step. Not required.

## Positive Notes

- The two-plane guard rails (Grafana token only to `/api/tokens`, write token only to `/v1/logs`, never echo either value) precisely mirror the service's core invariant and the code's own no-log-the-token discipline.
- Testing deactivation by re-sending the *same* token+body and expecting `401` exercises the real SQL-level `active=1` invariant rather than a proxy for it.
- Task decomposition (scaffold → mint → write+readback → negative+deactivate → docs) with explicit dependencies is clean and each task maps to a verified code path.

## Context Gates

- **Architecture:** WARN (advisory) — no `.ai-factory/ARCHITECTURE.md` in this sub-repo. The plan respects the two-plane split and the label-blind guarantee from `CLAUDE.md`; no boundary violation. The script is a repo-root sibling of `Makefile`, consistent with the existing tooling layout.
- **Rules:** No `.ai-factory/RULES.md` present — nothing to enforce.
- **Roadmap:** Correctly scoped to the proxy sub-repo (verifies the proxy's own write path), per the root routing rules that keep SDK/service work out of the root ROADMAP.

---

The one correctness bug from round 1 is fully addressed and every API assumption checks out against the current code. The two remaining items are optional polish, not blockers.

PLAN_REVIEW_PASS
