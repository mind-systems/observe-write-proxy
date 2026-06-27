# 08 — Grafana validation client

**Task:** ROADMAP → Phase 5 — Grafana-authenticated admin API → "Grafana validation client"
**Depth:** low

## Goal

A small client that answers one question for the admin plane: "is this Grafana token valid right now?" It calls the configured Grafana instance with the operator's token and resolves to a clean true/false, while reporting any inability to reach Grafana as an error the caller can map to a 5xx rather than a silent denial.

## Design

- New package `internal/grafana`. It is imported by `admin` only; `proxy` never imports it (write plane and admin plane stay disjoint).
- A `Client` type built once at startup:
  - `baseURL string` — the resolved Grafana base URL (default `http://localhost:3030`, from config), trailing slash normalised once at construction so request URLs do not double up.
  - `http *http.Client` — with an explicit request timeout (sane default, e.g. 10s) so a hung Grafana cannot pin the request goroutine.
- Constructor `NewClient(baseURL string) (*Client, error)` — validate/normalise the base URL once and fail fast on a malformed value.
- `Validate(ctx context.Context, token string) (bool, error)`:
  - Build a `GET` to `<baseURL>/api/user` with `http.NewRequestWithContext(ctx, …)` so the caller's deadline/cancellation propagates.
  - Set `Authorization: Bearer <token>` — the operator's Grafana token, passed straight through.
  - Map the response:
    - `200` → `(true, nil)` — the token authenticates to Grafana.
    - `401` / `403` → `(false, nil)` — a definite "not valid". This is a real verdict, not an error.
    - any other status (`5xx`, `404`, unexpected) → `(false, err)` — Grafana answered but not in a way that constitutes a verdict; treat as "could not validate", not "invalid".
    - a non-nil error from `client.Do` (connection refused, DNS, TLS, timeout) → `(false, err)`.
- **Decision — no role/permission gate.** Per canon, any valid Grafana token is accepted; the proxy has no user accounts and reads no role, org, or permission off the `/api/user` response. `200` alone means "allowed to administer". `/api/user` is chosen because it is the cheapest authenticated read that any valid token can perform, so it doubles as a pure validity probe without implying a privilege check.

## Edge cases / watch

- **Never log the operator token** — not in request logs, not in error strings. Errors returned must describe the failure (status, transport) without echoing the credential.
- **Respect ctx** — the call must abort on caller cancellation or deadline; do not run an independent timeout that ignores the passed context.
- **Don't follow redirects into other hosts** — a misconfigured or hostile Grafana could `30x` the `/api/user` call elsewhere and leak the `Authorization` header to a different host. Constrain redirect handling (e.g. a `CheckRedirect` that refuses cross-host hops) so the Bearer token is never replayed to an unexpected origin.
- **Transport failure is not "invalid"** — a `false` from a transport error and a `false` from a real `401` mean different things to the caller; the error return is what distinguishes them, so callers must inspect `err` before trusting the bool.

## Out of scope

- The middleware that extracts the header and decides the HTTP status from this verdict (task 09).
- Any caching/session of validation results — canon mandates per-request validation; this client performs one live call each time.
- Reading roles, orgs, or permissions from Grafana — explicitly excluded by the no-role-gate policy.

## Done when

- `Validate` returns `(true, nil)` for a real, currently-valid Grafana token.
- `Validate` returns `(false, nil)` for a bogus/expired token (Grafana answers `401`/`403`).
- `Validate` returns a non-nil error (caller will map to 5xx) when Grafana is unreachable or answers with an unexpected status, and never emits the token into logs.
