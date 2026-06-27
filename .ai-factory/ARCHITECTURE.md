# Architecture: Structured Modules (Technical Layers)

## Folder Structure

```
cmd/proxy/      # main: flag/env parsing, wiring, HTTP server start
internal/
├── proxy/        # write plane: /v1/logs handler, token check, pass-through forward to Loki
├── admin/        # admin plane: GUI + management API handlers, Grafana token validation
├── store/        # SQLite token store (pure-Go driver); the only stateful package
└── grafana/      # Grafana API client used by admin to validate operator tokens
web/              # static GUI assets served by the admin plane
```

Concrete package layout is a starting point — refine as the service takes shape.

## Dependency Rules

- `proxy/` → `store/` only (token lookup) + the upstream Loki URL from config
- `admin/` → `store/` (mint/list/revoke) + `grafana/` (validate operator)
- `store/` → nothing outside itself
- `grafana/` → nothing in this repo
- ❌ `proxy/` never imports `admin/` or `grafana/` — the write plane has no Grafana dependency
- ❌ no package parses or rewrites the forwarded OTLP body — the proxy is label-blind

## Request Flow

Write: `POST /v1/logs` → `proxy` reads `Authorization: Bearer` → `store` lookup (token must exist **and** be active) → ok: forward body **unchanged** to `…/otlp/v1/logs`; unknown or inactive: `401`.

Admin: any management request → `admin` reads the operator's Grafana token → `grafana` validates it (per request, no local session) → on success serves the GUI / mutates `store`.

## Two Auth Planes (frozen)

Write auth (proxy tokens, SQLite, instant revoke) and admin auth (Grafana token, validated per request) never mix. A Grafana token cannot authenticate a write; a write token cannot reach the admin plane. This separation is the architectural spine — code boundaries above enforce it.

## Token Store Shape

Per token: `name` (display-only, no authorization meaning), `token` (plaintext value), an `active` flag, plus creation metadata. The name is never matched against forwarded payloads. Revoking flips `active` to false and takes effect on the next write; rows are never deleted, so a token can be reactivated.
