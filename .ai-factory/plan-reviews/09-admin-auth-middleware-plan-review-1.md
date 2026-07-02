# Plan Review: 09 — Admin auth middleware

**Plan:** `.ai-factory/plans/09-admin-auth-middleware.md`
**Target:** `internal/admin/auth.go` (new), `cmd/proxy/main.go`
**Risk Level:** 🟢 Low

## Verification against the codebase

Every concrete assumption the plan makes was checked against the actual source:

- **`internal/proxy/auth.go` patterns exist as described.** The `tokenLookup` narrow-interface pattern (lines 14–16) and the `bearerToken` helper (lines 68–84, case-insensitive `Bearer` scheme, whitespace-trimmed, empty→`false`) are exactly what the plan tells the implementer to mirror/copy. The instruction to *copy* rather than *import* `internal/proxy` is correct and required.
- **`*grafana.Client` satisfies the proposed `grafanaValidator` interface.** `Client.Validate(ctx context.Context, token string) (bool, error)` (`internal/grafana/client.go:91`) matches the interface method verbatim. The client's documented return contract — `(true,nil)`=valid, `(false,nil)`=401/403, `(_,err)`=unexpected/transport — maps cleanly onto the plan's three-way status mapping.
- **`grafana.NewClient(cfg.GrafanaURL)` exists and returns `(*Client, error)`** (`client.go:44`), so the fail-fast wiring in Task 2 is real API.
- **`cfg.GrafanaURL` exists** (`internal/config/config.go:23`) and is already validated at load time.
- **`newMux` and the placeholder routes exist as stated.** `cmd/proxy/main.go:88` currently registers `/api/tokens` and `/api/tokens/` via `HandleFunc(handleNotImplemented)`. Switching them to `mux.Handle("/api/tokens", gate(http.HandlerFunc(handleNotImplemented)))` is the correct mechanical change. `/` and `/healthz` are indeed separate routes that the plan correctly leaves ungated.
- **Dependency direction matches `ARCHITECTURE.md`.** The declared rules (lines 20–23) are `admin/ → store/ + grafana/` and `proxy/ never imports admin/ or grafana/`. The plan's `admin → grafana` dependency and its explicit "do not import `internal/proxy`" instruction respect the frozen two-plane boundary exactly.

## Context Gates

- **Architecture (`ARCHITECTURE.md`):** PASS. Dependency rules, the two-plane split, and the "GUI/`/healthz` ungated, `/api/tokens` gated" routing all match the plan.
- **Rules (`.ai-factory/rules/base.md`):** PASS with one note. Naming (`internal/admin`, `auth.go` snake_case, `RequireGrafana` PascalCase constructor), the "planes stay separated in code" rule, and "never log token values" are all honored. **WARN (non-blocking):** `base.md` → Error Handling says "Grafana validation failure on the admin plane → `401`" and only prescribes 5xx for *Loki* failure — it is silent on the "Grafana unreachable" case. The plan's `503` for `(_, err)` is nonetheless correct: it follows the more specific decision in the spec note (§Design, "Decision: `503`") and ROADMAP (`502`/`503`), which take precedence over the terse base rule. The base rule could later be amended to cover the unreachable case, but this is not a plan defect.
- **Roadmap (`ROADMAP.md`):** PASS. Task maps to Phase 5 → "Admin auth middleware" (line 29). Status codes (`401` missing/invalid, `5xx` unreachable), the per-request no-session policy, and the public-GUI guard all match the roadmap line and spec note `09`. Task 2 correctly defers the real handlers to task 10.

## Critical Issues

None.

## Minor Observations (non-blocking)

1. **`newMux` signature + call site.** Task 2 says to "thread it into `newMux`", which requires adding a `gate func(http.Handler) http.Handler` parameter to `newMux` **and** updating its sole call site (`main.go:41`, `newMux(st, proxy.NewHandler(fwd))`). The plan implies both but only names the wrapping explicitly. A competent implementer will handle the call-site update, but it is worth being aware of. Also confirm `handleNotImplemented`/`newMux` stay in `package main` — no import cycle arises since `admin` imports neither.

2. **Middleware shape consistency.** The plan proposes `RequireGrafana(v grafanaValidator) func(http.Handler) http.Handler` (a decorator), whereas the existing write-plane gate uses `NewAuthMiddleware(lookup, next) http.Handler` (wraps `next` directly). Both are valid; the decorator form is arguably cleaner for building the gate once and applying it to multiple routes (`/api/tokens` and `/api/tokens/`), which is exactly this task's need. No conflict — just a deliberate, reasonable divergence from the sibling package's style.

3. **`grafana.NewClient` re-validates the URL** that `config.Load` already validated. This is redundant but harmless and matches the existing fail-fast pattern used for `NewForwarder`/`store.Open`. Fine as-is.

## Positive Notes

- The plan correctly enforces the architectural spine: copy `bearerToken` instead of importing `proxy`, and depend on a narrow local `grafanaValidator` interface rather than the concrete `*grafana.Client`. This mirrors the established `tokenLookup` pattern and keeps the planes decoupled.
- Security details are precise and correct: short-circuit missing/empty credentials **before** calling Grafana (no empty Bearer sent upstream), never log the raw token, generic error bodies, header-only inspection (body never read), and the deliberate `503`-vs-`401` distinction so "Grafana down" ≠ "token invalid".
- Scope discipline is good: placeholders stay in place, real handlers deferred to task 10, write-plane wiring untouched, GUI/`/healthz` left public with a stated rationale.
- No DB/schema changes involved, so no migration is needed — correctly absent.

PLAN_REVIEW_PASS
