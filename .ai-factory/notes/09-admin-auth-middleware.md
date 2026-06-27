# 09 — Admin auth middleware

**Task:** ROADMAP → Phase 5 — Grafana-authenticated admin API → "Admin auth middleware"
**Depth:** low

## Goal

A middleware that fronts the `/api/tokens` management endpoints and lets a request through only when it carries a currently-valid Grafana token. It extracts the operator's Bearer credential, validates it live against Grafana on every request, and turns the three possible outcomes — missing/malformed, invalid, and "couldn't reach Grafana" — into the right HTTP status so management is gated without the proxy ever holding a session of its own.

## Design

- Lives in `internal/admin`. It depends on `internal/grafana` (the task-08 `Client`) and on nothing in the write plane — `proxy` is never imported here.
- A middleware constructor, e.g. `RequireGrafana(g *grafana.Client) func(http.Handler) http.Handler` (or an equivalent `http.Handler` wrapper), built once at startup with the shared Grafana client and wrapped only around the `/api/tokens` handlers.
- Per request, in order:
  1. Read `Authorization`. Require the `Bearer ` scheme and a non-empty token; a missing header or a malformed/empty value → `401` (do not call Grafana for a request that has no credential to check).
  2. Call `g.Validate(r.Context(), token)` **per request** — pass the request context so cancellation/deadline propagates. There is no proxy-side session, cookie, or cache; the GUI re-sends the token on every call by design.
  3. Map the result:
     - `(true, nil)` → call the wrapped handler.
     - `(false, nil)` → `401` (Grafana gave a definite "not valid").
     - `(_, err)` → Grafana is unreachable or misbehaving; this is a proxy/upstream fault, not the operator's. Return a `5xx` — **Decision: `503` (Service Unavailable)** as the default, since the dependency being down is a transient availability problem; `502` is acceptable if the failure is specifically a bad upstream response. The point is a 5xx, never a silent `401`, so the operator can tell "wrong token" from "Grafana down".
- **Two planes never mix.** This middleware validates a *Grafana* token and gates *management* only. It never reads, mints, or checks a proxy write token, and it is never mounted on `POST /v1/logs`. The write plane keeps its own per-request proxy-token check; the two share no auth state.

## Edge cases / watch

- **The GUI page (`/`) is public.** Only the `/api/tokens` collection and `/api/tokens/<id>` item routes are wrapped by this middleware. `/` serves the static page whose only job is to collect the operator's Grafana token — gating it would create a chicken-and-egg lock-out. `/healthz` is likewise ungated.
- **Don't pre-judge before validating** — beyond the cheap header-shape check, a well-formed token that Grafana rejects is the `Validate` call's job to catch; don't try to guess validity locally.
- **Distinguish "no token" from "bad token"** — both are `401`, but only the second one should have cost a Grafana round-trip; short-circuit the missing-header case to avoid a pointless upstream call (and to avoid sending an empty Bearer to Grafana).
- **Never log the token** — the middleware sees the raw credential; keep it out of access logs and error messages (the task-08 client has the same rule).

## Out of scope

- The Grafana call itself (task 08 — the `grafana.Client`).
- The token-management handlers this middleware wraps (task 10).
- Serving the static GUI page and `/healthz` (their own tasks); this note only states that they sit outside the gate.

## Done when

- A management request carrying a valid Grafana token reaches the wrapped handler.
- A request with a missing or malformed `Authorization` header → `401` with no Grafana call made.
- A request whose token Grafana rejects → `401`.
- With Grafana down, a management request → `5xx` (not `401`), so "Grafana unreachable" is never mistaken for "token invalid".
