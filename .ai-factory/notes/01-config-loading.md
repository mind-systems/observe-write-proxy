# 01 — Config loading

**Task:** ROADMAP → Phase 1 — Service foundation → "Config loading"
**Depth:** low

## Goal

The proxy reads its four runtime settings — listen address, Loki base URL, Grafana base URL, and SQLite path — from the environment with canon defaults, validates them up front, and hands `main` a single validated `Config` value. Anything malformed (an unparseable URL, an empty listen address or DB path) stops the process before it serves a request, so misconfiguration surfaces at startup rather than as a silent forwarding failure later.

## Design

- New package `internal/config`, exposing a `Config` struct and a single constructor `Load() (*Config, error)`.
- `Config` fields (all settings the rest of the service needs):
  - `ListenAddr string` — the address the HTTP server binds.
  - `LokiURL string` — the Loki **base** URL, stored as given (e.g. `http://localhost:3100`). It is deliberately the base, **not** the OTLP endpoint — see the join note below.
  - `GrafanaURL string` — the Grafana base URL used by `internal/grafana` to validate admin tokens.
  - `DBPath string` — filesystem path to the SQLite file backing `internal/store`.
- Sources and precedence: each setting is resolved as **flag overrides env overrides default**. `Load()` defines a flag per setting (`-listen`, `-loki-url`, `-grafana-url`, `-db-path`) whose *default value* is the corresponding env var if set, otherwise the canon default. After `flag.Parse()`, a flag passed on the command line wins; if absent, the flag carries the env value; if neither, the canon default. **Decision:** flag > env > default, because an operator running the binary by hand should be able to override a baked-in environment for a one-off run without editing it.
- Env vars and canon defaults:
  - `PROXY_LISTEN` → `ListenAddr`, default `:4318`.
  - `LOKI_URL` → `LokiURL`, default `http://localhost:3100`.
  - `GRAFANA_URL` → `GrafanaURL`, default `http://localhost:3030`.
  - `DB_PATH` → `DBPath`, default `./proxy.db`.
- Validation, fail-fast, inside `Load()` after resolution:
  - `ListenAddr` must be non-empty (a bare `:4318` is valid — only emptiness is rejected here; binding is the server's job, task 02).
  - `LokiURL` and `GrafanaURL` must each parse via `net/url.Parse` **and** carry a scheme and host (a value like `localhost:3100` parses without error but yields no host, so check `u.Scheme != ""` and `u.Host != ""`). This catches the common "forgot the `http://`" mistake that `url.Parse` alone would let through.
  - `DBPath` must be non-empty.
  - On any failure return a wrapped error naming the offending setting and the bad value (never silently fall back). `main` surfaces it and exits non-zero (task 02 owns the exit).
- Forward-target derivation — where the join happens: the canon forward target is `<LokiURL>/otlp/v1/logs`. `Config` stores only the **base** `LokiURL`; it does **not** pre-join. The join is performed once in `internal/proxy` when it builds its upstream target, using `url.Parse(base)` + `JoinPath("otlp", "v1", "logs")` (Go 1.19 `url.URL.JoinPath`) so a trailing slash on the configured base cannot produce `//otlp`. Keeping the base un-joined in config keeps a single normalization point and avoids two packages each massaging the slash.

## Edge cases / watch

- A scheme-less authority like `localhost:3100` is a valid `url.Parse` input but parses as `Scheme="localhost"`, `Opaque="3100"` with an empty `Host` — without the host/scheme check it would pass validation and then fail opaquely at forward time. The host+scheme assertion is the real guard, not the bare parse.
- Do not trim or rewrite a trailing slash on `LokiURL` inside config — leave it intact and let the proxy's `JoinPath` absorb it, so there is exactly one place responsible for the slash and config stays a dumb carrier.
- `flag` default-from-env means env is read at flag-definition time; ensure `Load()` reads env *before* defining the flags so the env value becomes the flag default rather than being checked afterward.

## Out of scope

- Actually opening the SQLite file or pinging Loki/Grafana — config only validates shape, it does not test connectivity.
- The proxy's construction of the OTLP endpoint URL (consumes `LokiURL`; lives in task 03+).
- HTTP server construction, signal handling, and the non-zero exit itself (task 02).

## Done when

- `config.Load()` returns a fully populated, validated `*Config` when env/flags are well-formed, applying the canon defaults whenever a setting is absent from both flag and env.
- A malformed URL, an empty listen address, or an empty DB path makes `Load()` return a clear error naming the setting and value, with no partially-initialized config leaking out.
- `LokiURL` is stored as the base only; no package other than `internal/proxy` constructs the `/otlp/v1/logs` target.
