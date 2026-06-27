# 07 — Bearer auth on /v1/logs

**Task:** ROADMAP → Phase 4 — Write authentication → "Bearer auth on /v1/logs"
**Depth:** medium

## Goal

Guard the OTLP write path so only requests carrying a known, active proxy token reach Loki. The `POST /v1/logs` forwarder from task 04 gains a Bearer-auth gate that checks the token store on every request, making token deactivation effective on the very next write with no restart or reload.

## Design

- Add an auth middleware in `internal/proxy` that wraps the existing `POST /v1/logs` handler from task 04. It takes the token store (the `Lookup` capability of `internal/store`) and the next `http.Handler`, and returns an `http.Handler`. Wiring stays in `cmd/proxy` where the route is registered: the forwarder is constructed first, then wrapped by this middleware before being mounted on `/v1/logs`. Dependency rule holds — `proxy`→`store` only.
- Header extraction: read `Authorization`. Split on the first space into scheme + credential; match the scheme case-insensitively against `bearer` and trim surrounding whitespace from the credential. A missing header, an empty header, a non-Bearer scheme, or an empty credential after trim → `401` with no body detail and the request is **not** forwarded.
- On a well-formed token, call `store.Lookup(token)` once **per request, with no caching**. The store contract: a token is valid only if a row exists AND `active=true` (per canon). Treat "row missing" and "row present but inactive" identically — both produce the same `401`. On a valid active token, invoke the next handler (the forwarder) unchanged; the request body is never read or parsed by this middleware (label-blind).
- A store/infrastructure error from `Lookup` (e.g. SQLite failure) → `500`, distinct from the auth `401`. This keeps a backend outage from masquerading as an auth failure.
- Logging: never log the token value. Auth-failure log lines (if any) record only the outcome and request metadata, never the credential or the forwarded payload.

**Decision:** the same opaque `401` is returned for malformed-header, unknown-token, and inactive-token cases, with no distinguishing body or header — so the proxy never reveals whether a token exists vs. is merely inactive. Constant-time comparison is deliberately **not** required: this is a single-operator local stack, and `Lookup` is a store query keyed on the token, not a byte-compare in the handler.

## Edge cases / watch

- `Authorization` present but scheme is not Bearer (e.g. `Basic`) → `401`, not a passthrough.
- Credential with leading/trailing whitespace must be trimmed before `Lookup`, or a valid token padded by a sloppy client fails spuriously.
- A token that exists but is inactive must NOT forward and must NOT return a different status than an unknown token — same `401`.
- Distinguish "not found / inactive" (→ `401`) from "store errored" (→ `500`); never collapse a store error into a `401`, which would silently drop writes during a DB hiccup as if the token were bad.
- Lookup is per request by design — do not add a cache or memoize results; caching would defeat next-write deactivation.

## Out of scope

- The forwarding logic itself (task 04) — this task only gates it.
- Admin-plane (Grafana) auth for the GUI and `/api/tokens` — that is a separate plane (tasks 08–10); the write plane never validates against Grafana.
- Token creation/deactivation mechanics and the store schema — owned by the store/admin tasks; this task only consumes `Lookup`.

## Done when

- A `POST /v1/logs` with a valid, active token forwards to Loki exactly as task 04 does.
- A request with a missing or malformed `Authorization` header gets `401` and does not forward.
- A request with an unknown token, and one with an inactive token, both get the identical `401`.
- Deactivating a token via the store causes the next write carrying it to fail with `401`, with no restart or reload.
- A store error during lookup surfaces as `500`, not `401`.
- No token value is ever written to logs.
