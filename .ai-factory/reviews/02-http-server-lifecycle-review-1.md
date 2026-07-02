# Code Review: 02 — HTTP server + lifecycle

## Summary

**Files reviewed (in full):** `cmd/proxy/main.go` (new, 96 lines) against `internal/config/config.go`, the plan `plans/02-http-server-lifecycle.md`, spec note `notes/02-http-server-lifecycle.md`, and `ARCHITECTURE.md`.
**Scope of diff:** one new source file (`cmd/proxy/main.go`) plus planning artifacts (plan, plan-reviews, plan `.json`). Only `main.go` is code.
**Verdict:** 🟢 No blocking findings. Builds, vets clean, and passes an end-to-end smoke test.

## Verification performed

- `go build ./...` — OK.
- `go vet ./cmd/...` — OK.
- Runtime smoke test (bound `:14318`):
  - `GET /healthz` → `200`.
  - `POST /v1/logs` → `501`.
  - `GET /api/tokens` → `501`.
  - `GET /api/tokens/abc` (subtree via trailing-slash registration) → `501`.
  - `GET /` (catch-all) → `501`.
  - `SIGTERM` → graceful `srv.Shutdown`, process exits `0`.

All four "Done when" criteria from the plan are met.

## Correctness assessment

- **Config fail-fast** — `config.Load()` error prints to stderr and `os.Exit(1)`. Matches Task 01's `(*Config, error)` contract and the fail-fast requirement.
- **Timeouts** — `ReadHeaderTimeout: 10s`, `ReadTimeout: 30s`, `WriteTimeout: 30s`, `IdleTimeout: 120s` — exactly the locked values.
- **Clean-shutdown non-error** — `ListenAndServe`'s `http.ErrServerClosed` is correctly excluded from the fatal path (`err != nil && err != http.ErrServerClosed`), avoiding the "clean exit looks like a crash" bug. `http.ErrServerClosed` is returned as a sentinel value directly, so `!=` comparison is correct here.
- **Second-signal escape hatch (plan review 1, critical #1)** — RESOLVED in code: `stop()` is called explicitly after `<-ctx.Done()` and before `srv.Shutdown(...)` (line 54), restoring default signal handling for the duration of the drain. The `defer stop()` remains as a harmless no-op.
- **Shutdown context parent (plan review 1, note #2)** — RESOLVED in code: `shutdownCtx` derives from `context.Background()` (line 60), not the already-`Done` `ctx`, so the drain gets its full 10s bound. The inline comment documents the reasoning so it isn't "fixed" back later.
- **Route table** — `/healthz` real (`200` + `ok`); `/v1/logs`, `/api/tokens`, `/api/tokens/`, `/` all placeholder `501` via `http.Error`. Dual `/api/tokens` + `/api/tokens/` registration is correct for Go 1.19 `ServeMux`. `/` is the intentional catch-all. No method wildcard syntax used — POST enforcement is correctly deferred to the later `/v1/logs` handler.
- **`/healthz` dependency-free** — reads no `cfg`, no store, no upstream; reports process liveness only, as required.
- **Architecture boundary** — `cmd/proxy` imports only `internal/config`; no `proxy/`, `admin/`, `store/`, or `grafana/` dependency. Pure wiring/lifecycle, matching the `cmd/proxy` role.

## Non-blocking notes (forward-looking, already flagged in plan reviews — not actionable in this task)

- **`os.Exit(1)` inside the server goroutine skips deferred cleanup.** Correct now (no stateful resource to close). When the SQLite store lands (task 05), prefer signaling `main` via an error channel so a deferred `store.Close()` still runs. Verified benign here: on a bind failure the goroutine terminates the whole process with code 1 while `main` is parked on `<-ctx.Done()`; on clean shutdown `ListenAndServe` returns `ErrServerClosed` and the goroutine exits without triggering the branch.
- **`WriteTimeout` (30s) equals the planned forwarder client timeout (30s).** Zero margin; flag for task 03 so a legitimate near-30s Loki round-trip isn't cut at the same instant. Not this task's concern.

REVIEW_PASS
