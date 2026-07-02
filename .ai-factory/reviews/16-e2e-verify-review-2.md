# Code Review: 16-e2e-verify (round 2)

**Change under review:** new `verify.sh` (repo root) + `README.md` "End-to-end verification" section.
**Method:** `git diff HEAD` / `git status`; re-read `verify.sh` in full against review-1's findings; re-checked the plan and spec note. README diff unchanged since round 1.

## Round-1 findings: both resolved

- **#1 (Low) — read-back matched the shared `e2e-` prefix, not this run's marker.** Fixed. `verify.sh:159` now filters on the unique marker:
  ```
  --data-urlencode "query={service_name=\"e2e-verify\"} |= \"$MARKER\""
  ```
  `$MARKER` is `e2e-<epoch>` (digits + hyphen only), so it is safe both inside the LogQL string literal and in the shell double-quote escaping. A non-empty result now proves *this run's* line landed, eliminating the rerun-within-120s false PASS.
- **Minor — poll curl could abort under `set -e` on a transient Loki blip.** Fixed. `verify.sh:162` now appends `|| true`, so a timed-out or failed poll attempt falls through to the `sleep 2` and retries the remaining attempts instead of aborting the script.

## Re-review of the full script

Re-examined the whole flow for bugs, security, and correctness:

- **Auth-plane separation holds** — Grafana token only on `/api/tokens` (mint `:91`, PATCH `:191`, DELETE `:213`); write token only on `/v1/logs` (`:145`, `:201`). Bogus-token step (`:179`) and post-deactivation rewrite (`:201`) both correctly expect `401`, matching `proxy/auth.go`'s pre-body Bearer check.
- **Status assertions correct** — mint `= 201`; write and PATCH use `=~ ^2` (2xx range, anticipating Loki's `204` and the handler's `200`); negatives use exact `401`. `curl -sS` without `-f` lets HTTP error codes flow into `%{http_code}` rather than tripping `set -e`.
- **Timestamps** — `TS_NS` is current-nanosecond (`:110`, `:126-127`), so the record falls inside the `now-120s … now+10s` window; the two separate `date +%s` calls at `:109-110` can differ by at most 1s, which is immaterial to the window.
- **Marker match grep** (`:163`) — with the marker-scoped query, `"values":[["` appears only when a matching stream returns entries; empty results (`"result":[]`) correctly miss and retry. Depends on Loki's compact JSON, which its HTTP API emits.
- **Secret hygiene** — neither `GRAFANA_TOKEN` nor the minted token is ever echoed; no `set -x`; `mktemp` response file is `trap`-cleaned on EXIT.
- **`json_field`** — extracts the string fields `token`/`id` via `jq` (preferred) or a `grep`+`sed` fallback; empty-result guard at `:103-105` fails fast if either is missing.

### Non-blocking observation (not a bug)

- The read-back PASS/FAIL line (`:171`, `:173`) prints `$STATUS`, which still holds the *write's* HTTP code (last set at `:144`) — the `query_range` call captures only its body into `RESULT`, not a status code. So "read back … (HTTP 204)" shows the write's code beside a read-back outcome. Purely cosmetic; the PASS/FAIL verdict itself is driven by `FOUND`, which is correct. Not worth changing.
- On an early-step failure (`fail` calls `exit 1`), the best-effort `DELETE` cleanup at `:213` is skipped, so a failed run can leave an active `e2e-verify` token in the store. Consistent with the intended smoke-gate "abort on first failure" behavior and harmless (a test token an operator can delete via the GUI); noted only for completeness.

## Verdict

Both round-1 findings are correctly fixed and no new bugs, security issues, or correctness problems were found. The script is a sound end-to-end smoke gate; the two observations above are cosmetic and explicitly non-blocking.

REVIEW_PASS
