# Code Review: 16-e2e-verify (round 1)

**Change under review:** new `verify.sh` (repo root) + `README.md` "End-to-end verification" section.
**Method:** `git diff HEAD` / `git status`; read `verify.sh` and the README diff in full; cross-checked against the plan, the spec note (`.ai-factory/notes/16-e2e-verification.md`), and the code paths named in plan-review-1 (`internal/proxy/auth.go`, `internal/admin/tokens.go`, `internal/store/token.go`, `internal/config/config.go`).

## Summary

The script is well-structured and matches the plan closely: env-overridable endpoints with the correct canon defaults, a required-`GRAFANA_TOKEN` preflight with a token-type-specific message, current-nanosecond timestamps (the critical fix from plan-review-1 is present at `verify.sh:110`, `126-127`), 2xx-range assertion for the write/PATCH, `401` for bogus and post-deactivation writes, a `mktemp`+`trap` cleanup, best-effort `DELETE` cleanup, and no secret ever echoed. The two-plane discipline holds — the Grafana token is sent only to `/api/tokens`, the write token only to `/v1/logs`. `set -euo pipefail` is set; `curl -sS` (no `-f`) correctly lets HTTP error codes flow into `-w '%{http_code}'` rather than aborting on 401.

One correctness finding below (low severity — it produces false PASS, never false FAIL), plus minor notes.

## Findings

### 1. (Low) Read-back matches the shared `e2e-` prefix, not this run's unique marker — a rerun within 120s can PASS even if the current write never landed

`verify.sh:159` filters the query on the prefix `|= "e2e-"`, and the success test at `:163` (`grep -q '"values":\[\["'`) only checks that *some* stream returned entries:

```
--data-urlencode 'query={service_name="e2e-verify"} |= "e2e-"'
```

But every run writes a body of `e2e-$(date +%s)` (`:109`) and the window is `now-120s … now+10s` (`:156-157`). The best-effort `DELETE` at `:213` removes the *token row*, not the log lines already ingested into Loki — those persist under Loki's own retention. So on a second run started within ~2 minutes of the first, the previous run's line is still inside the query window and still matches `|= "e2e-"`. Step 2 then reports **PASS** on the stale line even if the current run's `POST /v1/logs` silently failed to ingest.

Failure scenario: run `verify.sh`, then run it again 30s later against a proxy whose Loki forward is now broken (e.g. Loki wedged after the first run). Step 2's write may return 2xx (or the write assertion catches a non-2xx), but the read-back would find run 1's line and green-light the ingestion path that is actually down. The smoke gate — the whole point per the header comment at `:51-52` — gives a false all-clear.

This traces to the spec note, which literally specifies `|= "e2e-"` (line 26/45) while *also* stating the marker should be unique "so the read-back query can find exactly this line" (line 24) — the implementation faithfully copied the prefix and inherited the inconsistency.

**Fix:** filter on the unique marker, not the prefix:

```bash
--data-urlencode "query={service_name=\"e2e-verify\"} |= \"$MARKER\""
```

`$MARKER` is already computed and contains no LogQL metacharacters, so it drops in directly. This makes Step 2 verify *this run's* line and removes the rerun false-positive with no other change.

## Minor notes (non-blocking)

- **`FAIL` accumulator is dead state.** `fail()` sets `FAIL=1` and immediately `exit 1` (`:53`), so the trailing `if [ "$FAIL" -eq 0 ]` at `:217` is only ever reached with `FAIL=0`; the `else` branch (`:220-222`) is unreachable. Not a bug — the "exit on first failure" behavior is correct and intended — just redundant. Harmless to leave.
- **Polling curl under `set -e`.** If a `query_range` poll times out (`--max-time 10`, `:158`) or Loki is briefly unreachable, `curl` exits non-zero and `set -e` aborts the whole script mid-poll rather than retrying the remaining attempts. In practice the preflight and Step 2 write already establish the stack is up, so this is a narrow window; acceptable, but a `RESULT="$(curl … || true)"` would make the 5×2s retry loop robust to a transient blip, which is exactly what the retry is for.
- **`grep` JSON matching is compact-JSON-dependent.** Both the `json_field` grep fallback (`:62`) and the values check (`:163`) assume Loki/proxy emit compact JSON (no spaces after `:` / inside `[[`). Loki's HTTP API does return compact JSON, so this holds today; noting it as the reason `jq` is preferred where present.

## Verdict

The script is correct for the happy path and the two negative paths, and the plan-review-1 timestamp fix is correctly implemented. Finding #1 is a real but low-severity weakening of the read-back (false PASS only, and only on a fast rerun) — recommend the one-line marker fix before this is relied on as a smoke gate. README documentation is accurate and matches the script's behavior and status codes.
