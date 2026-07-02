# Plan: HTTP server + lifecycle

## Context
Turn `cmd/proxy/main.go` into the wiring and lifecycle owner: load config, build one `http.ServeMux`, run a single `*http.Server` with locked timeouts, answer `/healthz` with `200`, mount every other route as a routable placeholder, and shut down gracefully on `SIGINT`/`SIGTERM` without dropping in-flight requests.

## Settings
- Testing: no
- Logging: minimal
- Docs: no

## Tasks

### Phase 1: Skeleton and lifecycle

- [x] **Task 1: Bootstrap `main` — config load + fail-fast**
  Files: `cmd/proxy/main.go`
  Create package `main` (module `github.com/mind-systems/observe-write-proxy`, Go 1.19). In `main()`, call `cfg, err := config.Load()` (from `internal/config`, task 01, signature `Load() (*Config, error)`). On error, print to stderr (e.g. `fmt.Fprintln(os.Stderr, err)`) and `os.Exit(1)` — this is the fail-fast surfacing point for config validation. `cfg.ListenAddr` is the bind address used in Task 3. Keep `main` as the sole owner of the sequence: config → mux → server → shutdown. No auth, no store, no forwarding, no dependency wired into `/healthz`.

- [x] **Task 2: Route table on a single `http.ServeMux`** (depends on Task 1)
  Files: `cmd/proxy/main.go`
  Build `mux := http.NewServeMux()` and register the route table, planes split by path per canon:
  - `mux.HandleFunc("/healthz", ...)` — **real** handler: respond `200 OK` with a tiny static body (`ok`). Must not read `cfg`, a store, or any upstream — it reports that *this process* is alive.
  - `POST /v1/logs` — write plane (task 03+). Placeholder handler.
  - `/api/tokens` and `/api/tokens/` — register **both** patterns (ServeMux treats them as distinct); the exact path is the collection, the trailing-slash form is the subtree (`/api/tokens/<id>`, parsed by a later task). Both route to the placeholder now.
  - `/` — GUI static serving (task 11+). Placeholder; also the catch-all, so it must exist to avoid a bare `404` default.
  Placeholders return a clear "not implemented yet" response with status `501` (never a silent `200`) so a half-wired build is obvious. Under Go 1.19's mux there are no method/path wildcards; register `/v1/logs` as a plain path and let the later handler enforce `POST` — do not rely on `"POST /v1/logs"` pattern syntax.

- [x] **Task 3: `*http.Server` with locked timeouts + graceful shutdown** (depends on Task 2)
  Files: `cmd/proxy/main.go`
  Construct `srv := &http.Server{Addr: cfg.ListenAddr, Handler: mux}` with these locked timeouts (import `time`):
  - `ReadHeaderTimeout: 10 * time.Second`
  - `ReadTimeout: 30 * time.Second`
  - `WriteTimeout: 30 * time.Second`
  - `IdleTimeout: 120 * time.Second`
  Lifecycle:
  - At the top of `main` (before starting the server) set up `ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)`; `defer stop()`.
  - Run `srv.ListenAndServe()` in a goroutine; a returned error that is **not** `http.ErrServerClosed` is fatal (log to stderr + exit non-zero). Do **not** treat `http.ErrServerClosed` as a failure — a clean `Shutdown` also returns it.
  - Block on `<-ctx.Done()` until a signal arrives.
  - **On signal, call `stop()` explicitly — before `Shutdown`** — to restore default signal handling for the duration of the drain. `defer stop()` alone is insufficient: it runs only when `main` returns, i.e. *after* `Shutdown` has finished, so during a hung drain the `NotifyContext` handler is still installed and a second `SIGINT` merely re-cancels the already-cancelled `ctx` instead of force-killing. Calling `stop()` here is what makes the operator's second-`SIGINT` escape hatch actually terminate the process. (Keep the `defer stop()` too; a second explicit call is a harmless no-op.)
  - Then create a bounded `shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)` (`defer cancel()`) and call `srv.Shutdown(shutdownCtx)`. **Use `context.Background()` as the parent, not `ctx`** — `ctx` is already `Done` (it's what just unblocked `<-ctx.Done()`), so deriving from it would make `shutdownCtx` cancelled from birth, causing `Shutdown` to return immediately and drop in-flight requests. `Shutdown` stops accepting new connections and waits for in-flight handlers. If it returns an error (drain timed out), log it and exit non-zero; otherwise exit `0`.

  Sketch:
  ```go
  <-ctx.Done()
  stop() // restore default signal handling: a 2nd SIGINT now force-kills a hung drain
  shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
  defer cancel()
  if err := srv.Shutdown(shutdownCtx); err != nil {
      // log to stderr + exit non-zero
  }
  ```

## Done when
- `go run ./cmd/proxy` starts and binds `cfg.ListenAddr`; a config error instead exits non-zero with a clear message.
- `GET /healthz` returns `200`.
- `/v1/logs`, `/api/tokens`, `/api/tokens/`, and `/` are routable placeholders (`501`), not `404`s.
- `SIGINT`/`SIGTERM` triggers `srv.Shutdown`, in-flight requests complete within the 10s bound, and the process exits `0`; a normal shutdown is not misreported as an error.
