# Plan Review: 02 — HTTP server + lifecycle

## Code Review Summary

**Files Reviewed:** plan `02-http-server-lifecycle.md` against `internal/config/config.go`, spec note `02-http-server-lifecycle.md`, `ARCHITECTURE.md`, `ROADMAP.md`, `go.mod` (Go 1.19)
**Risk Level:** 🟡 Medium — one correctness gap in the stated shutdown behavior; everything else is sound and codebase assumptions verified.

### Context Gates

- **Architecture (`.ai-factory/ARCHITECTURE.md`):** ✅ PASS. The plan keeps `cmd/proxy` as pure wiring/lifecycle owner (config → mux → server → shutdown) with **no** auth, store, forwarding, or Grafana dependency — exactly matching the `cmd/proxy/` role and the dependency rules. `/healthz` is deliberately dependency-free. No boundary violation.
- **Rules (`.ai-factory/RULES.md`):** ⚠️ WARN — file not present (optional). No explicit convention violations detected against `CLAUDE.md` house rules (English, no Docker, native binary intent all respected).
- **Roadmap (`.ai-factory/ROADMAP.md`):** ✅ PASS. Plan maps 1:1 to the Phase 1 "HTTP server + lifecycle" milestone and its spec note `notes/02-http-server-lifecycle.md`. Scope stays within the milestone (skeleton + lifecycle only); later routes are correctly deferred as placeholders.

### Codebase assumptions — verified

- `config.Load() (*Config, error)` exists with that exact signature (`config.go:62`). ✅
- `cfg.ListenAddr` field exists and is the bind address (`config.go:17,83`). ✅
- Module path `github.com/mind-systems/observe-write-proxy`, Go 1.19 — matches `go.mod`; installed toolchain is `go1.19.3`. ✅
- `signal.NotifyContext`, `context.WithTimeout`, `http.Server.Shutdown`, `http.ErrServerClosed` all exist in Go 1.19. ✅
- Go 1.19 `http.ServeMux` has **no** method/path wildcards — the plan correctly registers `/v1/logs` as a plain path and forbids `"POST /v1/logs"` pattern syntax. ✅
- `cmd/proxy/main.go` does not yet exist (`cmd/` is empty) — the plan creates it, consistent with "Nothing serves HTTP yet." ✅

### Critical Issues

**1. Second-signal force-kill escape hatch will not work with only `defer stop()` (Task 3).**
The plan states (line 40): *"Do not swallow signals beyond the first — `stop()` restores default handling so a second `SIGINT` force-kills a hung drain."* The spec note repeats this claim. But the plan only prescribes `defer stop()` (line 36). A deferred `stop()` runs when `main` **returns** — i.e. *after* `srv.Shutdown` has already completed. During the drain (while `Shutdown` blocks), `stop()` has not yet run, so `signal.NotifyContext`'s handler is still installed: a second `SIGINT` is captured by Go's signal machinery and merely re-cancels an already-cancelled context — it does **not** revert to default (terminate) behavior. The operator's escape hatch for a hung drain therefore does not exist.

Fix: call `stop()` explicitly immediately after `<-ctx.Done()` unblocks and before `srv.Shutdown(...)`, in addition to (or instead of) the `defer`. Only then is default signal handling restored for the duration of the drain, making a second `SIGINT` force-kill as described.

```go
<-ctx.Done()
stop() // restore default signal handling: a 2nd SIGINT now force-kills a hung drain
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
_ = srv.Shutdown(shutdownCtx)
```

### Non-blocking Notes

**2. Plan correctly diverges from the spec note on the shutdown-context parent — keep it that way.**
The plan (line 39) uses `context.WithTimeout(context.Background(), 10*time.Second)` for `shutdownCtx`. The spec note instead writes `context.WithTimeout(ctx, 10*time.Second)`, where `ctx` is the signal context that is **already `Done`** at that point (it's what just unblocked `<-ctx.Done()`). Deriving the shutdown timeout from an already-cancelled parent would make `shutdownCtx` cancelled from birth, causing `srv.Shutdown` to return immediately and drop in-flight requests — defeating the whole task. **The plan's `context.Background()` is the correct choice; do not "reconcile" it back toward the note.** Worth an explicit callout in the plan so the implementer doesn't copy the note's parent context. (WARN — note bug, plan is right.)

**3. Fatal `ListenAndServe` path via `os.Exit` skips deferred cleanup (minor, forward-looking).**
Exiting the process from the server goroutine on a non-`ErrServerClosed` error is fine now (no resources to release). But later milestones add a SQLite store whose `Close()` will want to run on exit. Consider signaling `main` (a buffered error channel or `errgroup`) rather than `os.Exit` inside the goroutine, so the established pattern still runs deferred cleanup once there is state to close. Optional at this stage.

**4. `WriteTimeout` (30s) equals the planned forwarder client timeout (30s) — zero margin (forward-looking).**
The locked `WriteTimeout: 30s` is set here, while Task 03's forwarder is specified with its own 30s client timeout. Equal values mean a legitimate near-30s Loki round-trip risks the server cutting the response at the same instant. The note acknowledges "the server envelope must not be tighter than the proxy's forward timeout" — equal is not tighter, so this is acceptable, but a small margin (e.g. server slightly larger than forwarder) would be safer. Not actionable in this task; flag for Task 03.

### Positive Notes

- Route table is precise for Go 1.19: dual `/api/tokens` + `/api/tokens/` registration, plain-path `/v1/logs`, `/` as intentional catch-all, and `501` placeholders (never a silent `200`) so a half-wired build is obvious.
- `/healthz` correctly kept dependency-free — it reports process liveness, not upstream readiness, and won't block during shutdown.
- Timeout set is complete and well-reasoned (`ReadHeaderTimeout` < `ReadTimeout` to bound slow-header clients while allowing large OTLP batch bodies).
- Correctly handles `http.ErrServerClosed` as a non-error on clean shutdown, avoiding the classic "clean exit looks like a crash" bug.
- Fail-fast config surfacing (`stderr` + `os.Exit(1)`) matches Task 01's error contract and the "Done when" acceptance criteria.
- Task dependencies (1 → 2 → 3) are ordered correctly and each names the single file it touches.
