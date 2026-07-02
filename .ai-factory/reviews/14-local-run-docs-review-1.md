# Review: Local run docs (14)

## Scope
Docs-only milestone. The only code change is `README.md` (new "Configuration" and "Pointing an SDK at the proxy" sections, plus an expanded "Running it"). No source, config, or build changes — no runtime, migration, or type surface to break.

## Verification of factual claims
Every technical claim in the new README prose was checked against the actual code:

- **Env var names** (`PROXY_LISTEN`, `LOKI_URL`, `GRAFANA_URL`, `DB_PATH`) — match `internal/config/config.go` constants. ✓
- **Flag names** (`-listen`, `-loki-url`, `-grafana-url`, `-db-path`) — match the `fs.String(...)` definitions. ✓
- **Defaults** (`:4318`, `http://localhost:3100`, `http://localhost:3000`, `./proxy.db`) — match the `default*` constants. ✓
- **Precedence** ("a flag overrides the environment variable, which overrides the default") — matches `Load()`: env resolves into each flag's default, and an explicit flag then overrides it. ✓
- **`LOKI_URL` is a base URL and the proxy appends `/otlp/v1/logs`** — matches the `LokiURL` doc comment ("deliberately not the /otlp/v1/logs endpoint itself; the join happens later"). ✓
- **`PROXY_LISTEN` binds both planes on one host** — matches `newMux` mounting `/v1/logs`, `/api/tokens`, `/healthz`, `/` on the single server. ✓
- **SDK endpoint is the proxy's `/v1/logs`** — matches the mux route. ✓
- **Build/run commands** (`make build` → `./bin/proxy`, `make run` from source) — match the `Makefile` targets. ✓
- **Token is a proxy-owned write token, 401 on unknown/deactivated** — consistent with the write-plane Bearer auth (`proxy.NewAuthMiddleware`) and the design in CLAUDE.md.

## Style / spec compliance
- Prose, no tables of methods, no directory trees, English — matches house doc style and the plan's constraints.
- `<token>` placeholder used; no real token value embedded.
- Admin GUI / minting only forward-referenced, not documented here — matches out-of-scope boundary.

No correctness, security, or accuracy issues found.

REVIEW_PASS
