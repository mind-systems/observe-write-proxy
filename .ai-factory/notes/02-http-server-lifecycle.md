# 02 — HTTP server + lifecycle

**Task:** ROADMAP → Phase 1 — Service foundation → "HTTP server + lifecycle"
**Depth:** medium

## Goal

`cmd/proxy/main.go` becomes the wiring and lifecycle owner: it loads config, builds the route table on a single `http.ServeMux`, runs one `*http.Server` with sane timeouts, and shuts it down cleanly on `SIGINT`/`SIGTERM` so no in-flight request is dropped. This task delivers the runnable skeleton — `/healthz` answers `200` and the process exits gracefully on a signal — with every other route present only as a placeholder that later tasks replace.

## Design

- `cmd/proxy/main.go` owns the sequence:
  1. `cfg, err := config.Load()` (task 01). On error, print to stderr and `os.Exit(1)` — this is the fail-fast surfacing point for config validation.
  2. Build a single `mux := http.NewServeMux()` and register the route table (below).
  3. Construct `srv := &http.Server{Addr: cfg.ListenAddr, Handler: mux, ...timeouts}`.
  4. Run `srv.ListenAndServe()` in a goroutine; treat a returned error that is **not** `http.ErrServerClosed` as fatal (log + exit non-zero), since a clean shutdown also returns from `ListenAndServe`.
  5. Block on the shutdown context, then drain (below).
- Route table on the mux (planes split by path, per canon):
  - `POST /v1/logs` — write plane (task 03+). Placeholder handler here.
  - `/api/tokens` and `/api/tokens/` — admin management collection + item (task 09+). Placeholder. Note the trailing-slash registration so the subtree (`/api/tokens/<id>`) is routable; manual trailing-segment parsing is a later task's concern, not this one.
  - `/` — GUI static serving (task 11+). Placeholder; this is also the catch-all, so it must exist to avoid a bare `404` page being the default.
  - `/healthz` — health. **Real** handler: respond `200 OK` with a tiny static body (e.g. `ok`). No config or store dependency — it must answer even if Loki/Grafana/SQLite are down, since it reports that *this process* is alive, not its upstreams.
  - Placeholders return a clear "not implemented yet" response (`501` is reasonable) rather than a silent `200`, so a half-wired build is obvious during development.
- Timeouts on the `http.Server` (the "sane timeouts" the canon's never-break-ingestion rule wants at the edge):
  - `ReadHeaderTimeout` and `ReadTimeout` — bound slow-header / slow-body clients.
  - `WriteTimeout` — bound a stuck response.
  - `IdleTimeout` — reap idle keep-alive connections.
  - **Decision:** choose generous values (e.g. read/write ~30s, idle ~120s) because the write plane forwards OTLP log batches whose upstream Loki round-trip must fit inside `WriteTimeout`; the forward timeout proper is tuned in the proxy task, but the server envelope must not be tighter than it.
- Graceful shutdown:
  - `ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)` at the top; `defer stop()`.
  - After the server goroutine is launched, `<-ctx.Done()` blocks `main` until a signal arrives.
  - On signal, create a bounded `shutdownCtx` with `context.WithTimeout` (e.g. ~10s) and call `srv.Shutdown(shutdownCtx)`. `Shutdown` stops accepting new connections and waits for in-flight handlers to finish, which is what "without dropping in-flight requests" requires.
  - If `Shutdown` returns an error (timeout exceeded), log it and exit non-zero; otherwise exit `0`.

## Edge cases / watch

- `ListenAndServe` returns `http.ErrServerClosed` on a normal `Shutdown` — do **not** treat that as a failure, or every clean exit will look like a crash.
- `signal.NotifyContext` already resets default signal handling via `stop()`; a second `SIGINT` after shutdown begins reverts to the default (immediate) behavior, which is the desired escape hatch for an operator who wants to force-kill a hung drain — do not swallow signals beyond the first.
- `/healthz` must not depend on `cfg`, the store, or upstreams; wiring it through a dependency would make liveness lie about readiness and could block during shutdown.
- Register `/api/tokens` (no slash) and `/api/tokens/` (with slash) deliberately — `ServeMux` treats them as distinct patterns; the collection endpoint needs the exact path and the subtree needs the slash form. (Manual `<id>` parsing lands in the admin task; here both just route to the placeholder.)

## Out of scope

- Actual `/v1/logs` forwarding, token auth, the SQLite store, and the Grafana validation client (tasks 03–12).
- Real `/api/tokens` CRUD and the manual trailing-segment parse for `/api/tokens/<id>`.
- Serving real GUI assets from `web/`.
- Per-route auth middleware — the two auth planes are layered on in their own tasks.

## Done when

- `go run ./cmd/proxy` (or the built binary) starts, binds `cfg.ListenAddr`, and a config error instead exits non-zero with a clear message.
- `GET /healthz` returns `200`.
- Sending `SIGINT`/`SIGTERM` triggers `srv.Shutdown`, lets in-flight requests complete within the bounded timeout, and the process exits `0`; a normal shutdown is not misreported as an error.
- `/v1/logs`, `/api/tokens`, and `/` are routable placeholders, not `404`s.
