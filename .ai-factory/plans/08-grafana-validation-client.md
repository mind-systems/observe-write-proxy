# Plan: Grafana validation client

## Context
Add an `internal/grafana` package that answers one admin-plane question — "is this operator's Grafana token valid right now?" — by calling `GET <grafana>/api/user` and resolving to a clean true/false, while surfacing any inability to reach Grafana as an error (never a silent denial).

## Settings
- Testing: no
- Logging: minimal
- Docs: no

## Tasks

### Phase 1: Grafana validation client

- [x] **Task 1: Add the `grafana.Client` type and constructor**
  Files: `internal/grafana/client.go`
  Create package `internal/grafana` (package doc: admin-plane-only Grafana token validation; must never be imported by `internal/proxy`). Define `Client{ baseURL string; http *http.Client }`. Add `NewClient(baseURL string) (*Client, error)` that:
  - Parses/validates `baseURL` with `url.Parse` and rejects a value lacking scheme or host (mirror the fail-fast pattern in `internal/config`'s `isValidBaseURL` and `internal/proxy`'s `NewForwarder`); return a wrapped sentinel error (e.g. `ErrGrafanaURL`) on a malformed value.
  - Normalises the base URL once by trimming any trailing `/` (as `NewForwarder` does) so request URLs never double up.
  - Builds `http.Client` with an explicit request `Timeout` (10s constant, following `forwardTimeout` style) **and** a `CheckRedirect` that refuses cross-host hops so the operator's `Authorization: Bearer` is never replayed to a different origin on a `30x`.

- [x] **Task 2: Implement `Validate(ctx, token)`** (depends on Task 1)
  Files: `internal/grafana/client.go`
  Add `Validate(ctx context.Context, token string) (bool, error)`:
  - Build `GET <baseURL>/api/user` via `http.NewRequestWithContext(ctx, …)` so caller deadline/cancellation propagates (do not start an independent timeout that ignores `ctx`).
  - Set `Authorization: Bearer <token>` — the operator token passed straight through.
  - Map the response and always close `resp.Body`:
    - `200` → `(true, nil)`.
    - `401` / `403` → `(false, nil)` — a real "not valid" verdict, not an error.
    - any other status (`5xx`, `404`, unexpected) → `(false, err)` — "could not validate", caller maps to 5xx.
    - non-nil error from `client.Do` (refused/DNS/TLS/timeout) → `(false, err)`.
  - Never read or parse roles/orgs/permissions from the body — `200` alone means allowed (no role gate).
  - **Never emit the token** into any returned error string or log; error messages describe status/transport only.
