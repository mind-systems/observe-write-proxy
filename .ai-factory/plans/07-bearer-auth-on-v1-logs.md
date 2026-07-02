# Plan: Bearer auth on /v1/logs

## Context
Guard the open `POST /v1/logs` write path with a Bearer-auth middleware that checks the token store on every request, so only known active proxy tokens reach Loki and deactivation takes effect on the very next write.

## Settings
- Testing: no
- Logging: minimal
- Docs: no

## Tasks

### Phase 1: Auth middleware

- [x] **Task 1: Add Bearer auth middleware in `internal/proxy`**
  Files: `internal/proxy/auth.go`
  Add a new file defining the write-plane auth gate. It wraps the existing task-04 `*Handler` (an `http.Handler`) without editing `handler.go`.
  - Define a minimal lookup interface so `proxy` stays decoupled from `*store.Store` internals while honoring the allowed `proxy`→`store` dependency, e.g. `type tokenLookup interface { Lookup(token string) (store.Token, bool, error) }` (import `github.com/mind-systems/observe-write-proxy/internal/store`). This matches the existing `store.Lookup(token string) (Token, bool, error)` signature exactly.
  - Add `func NewAuthMiddleware(lookup tokenLookup, next http.Handler) http.Handler` returning an `http.HandlerFunc` (or small struct) that runs the gate then delegates to `next`.
  - Header extraction: read `r.Header.Get("Authorization")`. Split on the first space (`strings.Cut` / `strings.SplitN(v, " ", 2)`) into scheme + credential; match the scheme case-insensitively against `bearer` (`strings.EqualFold`); `strings.TrimSpace` the credential. A missing header, empty header, non-Bearer scheme, or empty credential after trim → write `401` and **return without calling `next`**. Use `http.Error(w, ...)` with a generic message and no distinguishing detail.
  - On a well-formed token, call `lookup.Lookup(token)` exactly once per request, no caching/memoization. Interpret the three results:
    - `ok == false, err == nil` (unknown **or** inactive token) → `401`, identical response to the malformed-header case (no oracle: same status, same body, no extra header).
    - `err != nil` (store/SQLite failure) → `500`, distinct from the auth `401`, so a backend hiccup never masquerades as an auth failure or silently drops a valid write.
    - valid active token → call `next.ServeHTTP(w, r)` unchanged. Do not read, buffer, or parse `r.Body` — the middleware is label-blind.
  - Logging: never log the token value or the request body. Any auth-failure log line records only outcome/metadata, never the credential.
  - Constant-time comparison is intentionally NOT required (single-operator local stack; `Lookup` is a keyed store query, not a handler byte-compare).

### Phase 2: Wiring

- [x] **Task 2: Open the store and wrap `/v1/logs` with the middleware** (depends on Task 1)
  Files: `cmd/proxy/main.go`
  Wire the store and the middleware into the existing config → forwarder → mux → server sequence.
  - After loading config and before building the mux, open the token store: `st, err := store.Open(cfg.DBPath)` (add the `internal/store` import); on error print to stderr and `os.Exit(1)`, matching the existing fatal-error pattern. Register `defer st.Close()` — or close it after `srv.Shutdown` returns — so the SQLite handle is released on shutdown.
  - Change `newMux` to accept the store (or an authenticated handler) and mount the wrapped handler on `/v1/logs`: construct the forwarder handler as today (`proxy.NewHandler(fwd)`), then `authed := proxy.NewAuthMiddleware(st, proxy.NewHandler(fwd))` and `mux.Handle("/v1/logs", authed)`. Update `newMux`'s signature and its single call site accordingly.
  - Leave `/healthz`, `/api/tokens`, `/api/tokens/`, and `/` exactly as they are — this task only gates the write path.
