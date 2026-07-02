# Plan Review 2: 02 — HTTP server + lifecycle

## Code Review Summary

**Files Reviewed:** plan `plans/02-http-server-lifecycle.md` against `internal/config/config.go`, spec note `notes/02-http-server-lifecycle.md`, `ARCHITECTURE.md`, `ROADMAP.md`, `rules/base.md`, `go.mod` (Go 1.19), plus prior review `plan-reviews/02-http-server-lifecycle-plan-review-1.md`
**Risk Level:** 🟢 Low — the one correctness gap raised in review 1 is fully closed; no new issues found.

### Context Gates

- **Architecture (`.ai-factory/ARCHITECTURE.md`):** ✅ PASS. `cmd/proxy/main.go` stays pure wiring/lifecycle (config → mux → server → shutdown). No `proxy/`, `admin/`, `store/`, or `grafana/` dependency is wired; `/healthz` is deliberately dependency-free. Matches the `cmd/proxy` role and the dependency rules; the two-plane split by path is honored (write plane `/v1/logs`, admin plane `/api/tokens`, GUI `/`).
- **Rules (`.ai-factory/rules/base.md`):** ✅ PASS. Package `main` is short/lowercase; file is `main.go`; no new dependency (cgo-free invariant preserved); planes stay separated by path; no token/payload logging introduced. Error handling at the HTTP boundary is deferred to later tasks as intended.
- **Roadmap (`.ai-factory/ROADMAP.md`):** ✅ PASS. Maps 1:1 to the Phase 1 "HTTP server + lifecycle" milestone and its spec note. Scope is skeleton + lifecycle only; `/v1/logs`, `/api/tokens`(+`/`), GUI are correctly left as `501` placeholders for later tasks.

### Codebase assumptions — verified

- `config.Load() (*Config, error)` exists with that exact signature (`config.go:62`). ✅
- `cfg.ListenAddr` field exists and is the bind address (`config.go:17,83`). ✅
- Module path `github.com/mind-systems/observe-write-proxy`, Go 1.19 — matches `go.mod`. ✅
- `signal.NotifyContext`, `context.WithTimeout`, `http.Server.Shutdown`, `http.ErrServerClosed`, `ReadHeaderTimeout` all exist in Go 1.19. ✅
- Go 1.19 `http.ServeMux` has no method/path wildcards — plan registers `/v1/logs` as a plain path and explicitly forbids `"POST /v1/logs"` pattern syntax. ✅
- `cmd/proxy/main.go` does not yet exist — plan creates it, consistent with "Nothing serves HTTP yet." ✅

### Resolution of prior review findings

- **Critical #1 (second-signal force-kill escape hatch)** — RESOLVED. Task 3 now prescribes calling `stop()` **explicitly** immediately after `<-ctx.Done()` and before `srv.Shutdown(...)`, with a precise rationale (deferred `stop()` runs only after `main` returns, i.e. after `Shutdown`, so it cannot restore default handling during the drain). The `defer stop()` is retained as a harmless no-op. The sketch encodes exactly this ordering. Correct.
- **Note #2 (shutdown-context parent)** — RESOLVED. Task 3 uses `context.WithTimeout(context.Background(), 10*time.Second)` and explicitly warns not to derive from the already-`Done` `ctx`. This overrides the spec note's `context.WithTimeout(ctx, ...)`, and the plan calls out the divergence so the implementer won't "reconcile" it back. Correct.
- **Note #3 (fatal `os.Exit` in goroutine skips deferred cleanup)** — forward-looking; still not applicable in this task (no stateful resource to close yet). Fine to leave for the store milestone.
- **Note #4 (`WriteTimeout` == forwarder timeout, zero margin)** — forward-looking; flagged for Task 03, not actionable here.

### Critical Issues

None.

### Non-blocking Notes

- **`WriteTimeout` (30s) equals the planned forwarder client timeout (30s).** Carried over from review 1; nothing to change in this task, but Task 03 should give the server envelope a small margin over the forwarder timeout so a legitimate near-30s Loki round-trip isn't cut at the same instant.
- **Fatal server-goroutine path uses `os.Exit`.** Acceptable now (no resources to release). When the SQLite store lands, consider signaling `main` via an error channel so deferred `Close()` still runs. Optional, forward-looking.

### Positive Notes

- The shutdown sequence is now unambiguous: `<-ctx.Done()` → explicit `stop()` → bounded `shutdownCtx` from `context.Background()` → `Shutdown`, with the two easy-to-get-wrong subtleties (signal restoration, non-cancelled parent) each spelled out inline.
- Route table is precise for Go 1.19: dual `/api/tokens` + `/api/tokens/` registration, plain-path `/v1/logs`, `/` as intentional catch-all, `501` placeholders (never a silent `200`).
- `/healthz` correctly kept dependency-free — reports process liveness, not upstream readiness.
- Timeout set is complete and reasoned (`ReadHeaderTimeout` < `ReadTimeout` for slow-header defense while allowing large OTLP batch bodies).
- `http.ErrServerClosed` correctly treated as a non-error on clean shutdown.
- Task dependencies (1 → 2 → 3) are ordered correctly; each names the single file it touches.

PLAN_REVIEW_PASS
