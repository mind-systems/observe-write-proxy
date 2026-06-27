# Base Rules

> Conventions for this Go service. Edit as the codebase grows.

## Naming Conventions

- Packages: short, lower-case, no underscores (`store`, `proxy`, `admin`)
- Files: `snake_case.go`; tests alongside as `*_test.go`
- Exported identifiers: `PascalCase`; unexported: `camelCase`
- Errors: sentinel values `ErrXxx`; wrap with `fmt.Errorf("...: %w", err)`

## Module Structure

- One static binary, no cgo — keep all dependencies pure-Go (notably the SQLite driver)
- Two planes stay separated in code as they are at runtime: write-path handling vs admin/management handling
- The token store is the only stateful component; access it through one package, not ad-hoc SQL across handlers
- The proxy is label-blind: forwarding code must pass the OTLP body through unchanged — never parse, enrich, or relabel it

## Error Handling

- HTTP boundary: invalid/missing write token → `401`; Grafana validation failure on the admin plane → `401`; upstream Loki failure → surface a `5xx`, do not silently swallow
- Internal errors are wrapped with context and logged; never panic in a request path

## Logging

- The proxy's own operational logging is separate from the traffic it forwards
- Do not log token values or full forwarded payloads
