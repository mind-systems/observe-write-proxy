# Plan: Admin auth middleware

## Context
Gate the admin plane's `/api/tokens` endpoints behind a middleware in `internal/admin` that validates the operator's Grafana Bearer token live against Grafana on every request, keeping the two auth planes fully separate.

## Settings
- Testing: no
- Logging: minimal
- Docs: no

## Tasks

### Phase 1: Middleware

- [x] **Task 1: Grafana-auth middleware in `internal/admin`**
  Files: `internal/admin/auth.go`
  Create package `internal/admin`. Add a `grafanaValidator` interface with the single method `Validate(ctx context.Context, token string) (bool, error)` — mirror the `tokenLookup` narrow-interface pattern in `internal/proxy/auth.go` so `admin` depends only on the shape it uses; `*grafana.Client` satisfies it. Add a constructor `RequireGrafana(v grafanaValidator) func(http.Handler) http.Handler` returning middleware that wraps a `next http.Handler`. Per request, in order:
  1. Extract the credential from `Authorization` using a local `bearerToken(header string) (string, bool)` helper (copy the case-insensitive `Bearer ` scheme parse from `internal/proxy/auth.go`; do **not** import `internal/proxy` — the planes never share code). Missing/malformed/empty → `401`, and return **without calling Grafana** (short-circuit, so no empty Bearer is sent upstream).
  2. Call `v.Validate(r.Context(), token)` — pass the request context so cancellation/deadline propagate; no session, cookie, or cache.
  3. Map results: `(true, nil)` → `next.ServeHTTP`; `(false, nil)` → `401`; `(_, err)` → `503` (Service Unavailable — a proxy/upstream fault, never a silent `401`, so "Grafana down" stays distinguishable from "token invalid").
  Never write the raw token into any log line or error message. Use generic `http.Error` bodies (e.g. `"unauthorized"`, `"grafana unreachable"`). This middleware only ever inspects headers — it never reads the request body.

### Phase 2: Wiring

- [x] **Task 2: Mount the gate around the `/api/tokens` routes** (depends on Task 1)
  Files: `cmd/proxy/main.go`
  In `main`, construct the Grafana client with `grafana.NewClient(cfg.GrafanaURL)` (fail fast on error, matching the existing `NewForwarder`/`store.Open` error handling), and build the middleware once via `admin.RequireGrafana(gclient)`. Thread it into `newMux` and wrap the existing `/api/tokens` and `/api/tokens/` handlers with it — for now those remain the `handleNotImplemented` placeholders (task 10 replaces the placeholders with the real management handlers behind this same gate). Use `mux.Handle("/api/tokens", gate(http.HandlerFunc(handleNotImplemented)))` and the same for `/api/tokens/`. Leave `/` (static GUI) and `/healthz` **ungated** — the GUI page only collects the token and gating it would lock the operator out. Do not touch the `/v1/logs` write-plane wiring; the two planes stay independent.
