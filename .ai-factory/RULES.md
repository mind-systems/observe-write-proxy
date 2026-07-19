# Project Rules

> Short, actionable rules and conventions for this project. Loaded automatically by /aif-implement.

## Module Structure

- The token store is the only stateful component; access it through one package, not ad-hoc SQL across handlers.

## Error Handling

- HTTP boundary: invalid/missing write token → `401`; Grafana validation failure on the admin plane → `401`; upstream Loki failure → surface a `5xx`, do not silently swallow.
- Internal errors are wrapped with context and logged; never panic in a request path.

## Logging

- The proxy's own operational logging is separate from the traffic it forwards.
- Do not log token values or full forwarded payloads.
