# Code Review: 09 — Admin auth middleware

**Scope:** `internal/admin/auth.go` (new), `cmd/proxy/main.go` (modified)
**Build:** `go build ./...` ✅ `go vet ./...` ✅

## What was reviewed

Read both changed files in full plus the surrounding code they depend on:
`internal/proxy/auth.go` (the sibling pattern being mirrored), `internal/grafana/client.go`
(the validator being consumed), and the rest of `cmd/proxy/main.go`.

## Correctness

The middleware implements the three-way outcome mapping exactly as specified and as the
`grafana.Client.Validate` contract requires:

- Missing/malformed/empty `Authorization` → `401`, returned **before** calling Grafana —
  no empty Bearer is ever sent upstream. Correct short-circuit.
- `Validate` returns `(_, err)` → `503`, kept distinct from the invalid case so
  "Grafana unreachable" never masquerades as "token invalid". Matches the spec decision.
- `Validate` returns `(false, nil)` → `401`.
- `Validate` returns `(true, nil)` → delegates to `next`.

`Validate` is called with `r.Context()`, so client cancellation/deadline propagates. No
session, cookie, or cache — the per-request check is what makes Grafana-side revocation
effective on the next admin call.

## Security

- **Two planes stay separate.** `internal/admin` does not import `internal/proxy`;
  `bearerToken` is copied with a comment stating why, honoring the frozen architectural
  boundary. Dependency direction matches `ARCHITECTURE.md` (`admin → grafana`, never
  `proxy`).
- **Token never leaked.** The raw credential is not written to any log line or error
  body; error responses are generic (`"unauthorized"`, `"grafana unreachable"`). The
  request body is never read — the gate inspects headers only.
- **Routing exposes no ungated admin path.** Both `/api/tokens` (exact) and
  `/api/tokens/` (subtree) are wrapped by the gate. `/` and `/healthz` are intentionally
  ungated; `/` only serves the token-collecting GUI, and gating it would lock the
  operator out. The `/` catch-all can only receive non-admin paths, all of which return
  the `handleNotImplemented` 501 placeholder — no admin functionality is reachable
  unauthenticated. `/v1/logs` write-plane wiring is untouched.

## Wiring

`grafana.NewClient(cfg.GrafanaURL)` is constructed with the same fail-fast pattern as
`NewForwarder`/`store.Open`. The gate is built once via `admin.RequireGrafana(gclient)`
and threaded into `newMux` as a `func(http.Handler) http.Handler` parameter; the sole
call site is updated. The `newMux` doc comment was updated to describe the new gating.
The decorator shape (vs. the write plane's `NewAuthMiddleware(lookup, next)`) is a
reasonable, deliberate divergence that suits applying one gate to two routes.

## Notes (non-blocking)

- No DB/schema changes, so no migration concern — correctly absent.
- Placeholders (`handleNotImplemented`) remain in place; the real management handlers are
  correctly deferred to task 10, which will sit behind this same gate.

No correctness, security, or runtime issues found.

REVIEW_PASS
