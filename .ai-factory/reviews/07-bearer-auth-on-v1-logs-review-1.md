# Code Review: 07-bearer-auth-on-v1-logs

## Scope
Code changes on the current branch for the Bearer-auth gate on `POST /v1/logs`.

**Files reviewed in full:**
- `internal/proxy/auth.go` (new)
- `cmd/proxy/main.go` (modified)
- Supporting context read in full: `internal/proxy/handler.go`, `internal/proxy/forwarder.go`, `internal/store/token.go`, `internal/store/store.go`, `internal/config/config.go`

**Verification:** `go build ./...` and `go vet ./...` both pass.

## Findings

None. The implementation is correct, matches the spec, and honors the architectural constraints.

### Correctness — verified against the code

- **Three-way `Lookup` result mapping is exact.** `bearerToken` returns `ok=false` for a missing/empty header, a header with no space-separated scheme, a non-Bearer scheme, or an empty-after-trim credential → `401`. `Lookup` returning `ok=false, err=nil` (unknown *or* inactive, since the `active=1` filter lives in SQL — `token.go:107`) → `401`. `err!=nil` → `500`. Valid active token → `next`. This is the precise mapping the spec requires and avoids the "store error masquerades as auth failure" trap.
- **No-oracle discipline holds.** Malformed-header and unknown/inactive-token paths both write `http.Error(w, "unauthorized", 401)` — identical status, body, and headers. The store-error path is a distinct `"internal error"` / `500`. A caller cannot distinguish "no such token" from "inactive token".
- **Per-request lookup, no cache.** `ServeHTTP` calls `m.lookup.Lookup` fresh on every request; no memoization anywhere — deactivation takes effect on the very next write, as required.
- **Label-blind preserved.** The middleware inspects only the `Authorization` header; it never reads, buffers, or parses `r.Body`.
- **No token leak to Loki.** Although `next.ServeHTTP(w, r)` receives the original request (carrying `Authorization`), `Forwarder.Forward` (`forwarder.go:76`) builds a fresh outbound request and copies only `Content-Type`/`Content-Encoding`. The inbound proxy token is never propagated upstream.
- **No token in logs.** The middleware logs nothing; error responses carry only generic strings.
- **Header parsing is robust.** `strings.Cut(header, " ")` splits on the first space; `strings.EqualFold` gives case-insensitive `bearer` matching; `strings.TrimSpace` strips a padded credential before lookup and rejects empty-after-trim. `"Bearer"` with no space → `found=false` → `401`.
- **Variable reuse is valid Go.** `_, ok, err := m.lookup.Lookup(token)` redeclares `err` (new) and reassigns `ok`; `go vet` clean.

### Wiring

- **Signatures match.** `store.Open(cfg.DBPath) (*Store, error)`, `(*Store).Close()`, `(*Store).Lookup(string) (Token, bool, error)`, and `proxy.NewHandler(fwd) *Handler` are all used correctly. `*store.Store` satisfies the unexported `tokenLookup` interface.
- **Dependency rule honored.** `proxy` imports `store` only (the allowed edge); nothing pulls in `admin`/`grafana`. The `tokenLookup` interface keeps `proxy` coupled to just the `Lookup` shape.
- **Store lifetime.** `defer st.Close()` runs at `main` return, after `srv.Shutdown` drains in-flight requests, so no in-flight lookup can race the close. The fatal `os.Exit(1)` paths skip the defer, but the process is terminating and the OS reclaims the handle — not a leak of concern.
- **No migration needed.** The `tokens` table and `active` column already exist and `Lookup` already filters `active=1`; this is pure read-path gating with no schema change.

### Behavior note (not a defect)

With the middleware outermost, an unauthenticated `GET /v1/logs` now returns `401` (auth) rather than the handler's `405` (method). This is acceptable and arguably preferable — auth precedes method disclosure — and is consistent with the plan.

REVIEW_PASS
