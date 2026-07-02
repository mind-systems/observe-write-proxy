# Plan Review: 08 — Grafana validation client

**Plan:** `.ai-factory/plans/08-grafana-validation-client.md`
**Files Reviewed:** plan + note 08/09, `internal/config/config.go`, `internal/proxy/forwarder.go`, `internal/proxy/handler.go`, `cmd/proxy/main.go`, `ARCHITECTURE.md`, `go.mod`
**Risk Level:** 🟢 Low

## Context Gates

- **Architecture (`.ai-factory/ARCHITECTURE.md`):** ✅ PASS. The plan matches the module map exactly — `grafana/` is a new package "imported by admin to validate operator tokens", the dependency rule `grafana/ → nothing in this repo` is honored (plan uses stdlib only: `net/http`, `net/url`, `context`), and the hard boundary `❌ proxy/ never imports grafana/` is explicitly restated in Task 1's package doc. `GET /api/user` and per-request validation (no local session) both match the Admin control flow in ARCHITECTURE.
- **Rules (`.ai-factory/RULES.md`):** ⚠️ WARN — file not present. No convention violations to check against; nothing blocking.
- **Roadmap (`.ai-factory/ROADMAP.md`):** ✅ PASS. Note 08 anchors the task to "ROADMAP → Phase 5 — Grafana-authenticated admin API → Grafana validation client", so this `feat` work has explicit milestone linkage.

## Critical Issues

None. The plan is implementable as written.

## Correctness / Consistency Notes (non-blocking)

1. **Module path / Go version — verified.** `go.mod` is `github.com/mind-systems/observe-write-proxy`, `go 1.19`. Every construct the plan relies on (`http.NewRequestWithContext`, `http.Client.CheckRedirect`, `errors` sentinels, `url.Parse`) is available on 1.19. New package import path will be `internal/grafana`. ✅
2. **`Timeout` + `ctx` do not conflict.** Task 1 sets an explicit `http.Client{Timeout: 10s}` while Task 2 uses `http.NewRequestWithContext`. This is *not* the "independent timeout that ignores ctx" the note warns against — Go applies both bounds and whichever fires first wins, so the caller's deadline/cancellation still propagates. This mirrors `NewForwarder` (`forwardTimeout`) correctly. Worth a one-line comment in the code so a future reader doesn't "fix" the apparent redundancy.
3. **`CheckRedirect` is belt-and-suspenders (good).** Since Go 1.8 the default client already strips `Authorization` on a cross-host redirect, so the token-leak vector is partly closed by default. The plan's explicit cross-host refusal is still the right call — it turns a silent redirect into a hard error that lands in the `(false, err)` "could not validate" branch, which is the desired behavior. No change needed; just noting it's defense-in-depth, not the only line.
4. **Status mapping is complete and matches the consumer (note 09).** `200 → (true,nil)`, `401/403 → (false,nil)`, everything else + transport error → `(false, err)`. Note 09's middleware maps `(false,nil) → 401` and `(_,err) → 5xx`, so the two-return-value contract (caller must inspect `err` before trusting the bool) is the load-bearing detail and the plan states it explicitly. ✅
5. **Minor — drain body before close.** The plan closes `resp.Body` but never reads it. On a keep-alive client, closing an undrained body can prevent TCP connection reuse. `/api/user` responses are tiny and the admin plane is low-frequency, so this is a micro-optimization, not a defect. Optional: `io.Copy(io.Discard, resp.Body)` before close.
6. **Testing disabled — verification is manual.** `Settings: Testing: no`, yet note 08's "Done when" is a behavioral spec (valid → true, bogus → false, unreachable → err). With no unit test and Grafana's client behavior already spot-verified in the note (`glsa_…` → 200, bogus → 401), acceptance rests on manual/e2e checking. Acceptable given the low-frequency admin surface and the stdlib-only implementation, but flagging that the "Done when" conditions won't be auto-guarded.

## Positive Notes

- Security posture is strong and explicit: never emit the token into errors/logs, refuse cross-host redirects, and treat "couldn't reach Grafana" as an error (never a silent denial) — all three canon requirements are pinned in the tasks, not just the note.
- Fail-fast constructor pattern (`ErrGrafanaURL` sentinel, scheme+host validation, trailing-slash normalization) faithfully mirrors the established `isValidBaseURL` / `NewForwarder` conventions, keeping the codebase consistent.
- Scope discipline: no role/org/permission parsing, no caching/session, middleware deferred to task 09 — the plane split stays clean and the client stays a pure validity probe.

PLAN_REVIEW_PASS
