# Plan: Config loading

## Context
Add an `internal/config` package that resolves the proxy's four runtime settings (listen address, Loki base URL, Grafana base URL, SQLite path) from flags over env over canon defaults, validates them up front, and returns a single validated `*Config` so misconfiguration fails at startup rather than as a silent forwarding failure later.

## Settings
- Testing: no
- Logging: minimal
- Docs: no

## Tasks

### Phase 1: Config package

- [x] **Task 1: Define the `Config` struct and canon defaults**
  Files: `internal/config/config.go`
  Create package `config` (module `github.com/mind-systems/observe-write-proxy`, Go 1.19). Define the exported `Config` struct with fields `ListenAddr string`, `LokiURL string`, `GrafanaURL string`, `DBPath string` — doc-comment each per the spec (note that `LokiURL` is the **base** URL, deliberately not the `/otlp/v1/logs` endpoint; the join happens later in `internal/proxy`). Declare the canon defaults and env-var names as unexported constants: `PROXY_LISTEN`→`:4318`, `LOKI_URL`→`http://localhost:3100`, `GRAFANA_URL`→`http://localhost:3000`, `DB_PATH`→`./proxy.db`. Follow house naming (`snake_case.go` file, `PascalCase` exports).

- [x] **Task 2: Implement `Load()` with flag > env > default resolution** (depends on Task 1)
  Files: `internal/config/config.go`
  Add `func Load() (*Config, error)`. Read each env var **before** defining flags so the resolved env-or-default value becomes each flag's default (per the watch note — `flag` reads its default at definition time). Define one flag per setting (`-listen`, `-loki-url`, `-grafana-url`, `-db-path`) using a dedicated `flag.FlagSet` (do not rely on the global `flag.CommandLine`, so the function is self-contained), each defaulting to `envOr(<ENV>, <default>)`. Parse `os.Args[1:]`. The result: a CLI-passed flag wins, else the env value, else the canon default. Add a small unexported `envOr(key, def string) string` helper. Populate and return a `*Config` only after validation (Task 3) passes.

- [x] **Task 3: Fail-fast validation with wrapped, named errors** (depends on Task 2)
  Files: `internal/config/config.go`
  Before returning the `*Config`, validate the resolved values; on any failure return `nil` plus a wrapped error naming the offending setting and its bad value (use `fmt.Errorf("...: %w", ...)` with sentinel `ErrXxx` values per house error convention), leaking no partially-initialized config. Rules: `ListenAddr` must be non-empty (a bare `:4318` is valid — reject only emptiness; binding is task 02's job). `DBPath` must be non-empty. `LokiURL` and `GrafanaURL` must each parse via `net/url.Parse` **and** carry both a scheme and a host (`u.Scheme != "" && u.Host != ""`) — this rejects the scheme-less `localhost:3100` case that `url.Parse` alone accepts (it parses as `Scheme="localhost"`, `Opaque="3100"`, empty `Host`). Do **not** trim or rewrite a trailing slash on `LokiURL` — store it verbatim; the proxy's `JoinPath` is the single normalization point.
