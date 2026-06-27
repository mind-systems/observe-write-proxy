# Roadmap — observe-write-proxy

> A thin Go proxy that authenticates OTLP log writes (its own tokens, SQLite-backed, deactivated not deleted) and forwards valid traffic to Loki, with a small Grafana-authenticated admin surface for minting and toggling those tokens. One host, planes split by path.

## Milestones

### Phase 1 — Service foundation

- [ ] **Config loading** — No configuration layer exists yet. Add `internal/config` with `Config{ListenAddr, LokiURL, GrafanaURL, DBPath}` loaded by `Load()` from env (`PROXY_LISTEN`, `LOKI_URL`, `GRAFANA_URL`, `DB_PATH`) with flag override and canon defaults (`:4318`, `http://localhost:3100`, `http://localhost:3030`, `./proxy.db`); validate URLs parse and paths are non-empty, failing fast. The forward target `<LokiURL>/otlp/v1/logs` is joined later in `proxy`, not stored, to avoid double slashes. Spec: `.ai-factory/notes/01-config-loading.md`.
- [ ] **HTTP server + lifecycle** — Nothing serves HTTP yet. In `cmd/proxy/main.go` load config, build an `http.ServeMux`, run an `*http.Server` with read/write/idle timeouts, and shut down gracefully on `SIGINT`/`SIGTERM` via `signal.NotifyContext` + `server.Shutdown`. Add `/healthz`→200; mount `/v1/logs`, `/api/tokens`, `/` as placeholders filled by later tasks. Owns the skeleton and lifecycle only — no auth, store, or forwarding. Spec: `.ai-factory/notes/02-http-server-lifecycle.md`.

### Phase 2 — Write-path forwarding

- [ ] **Loki forwarder** — `internal/proxy` does not exist. Add a `Forwarder` holding the resolved `<loki>/otlp/v1/logs` target and a timeout-bounded `*http.Client`; `Forward(ctx, body, contentType, contentEncoding)` streams the request body through **unchanged** (label-blind — never read or parse it), copies `Content-Type`/`Content-Encoding`, relays Loki's status and body, and maps transport failures (refused/DNS/timeout) to `502`/`504`. Guard: no body buffering, no hop-by-hop headers. Spec: `.ai-factory/notes/03-loki-forwarder.md`.
- [ ] **/v1/logs route** — No write endpoint yet. Add the `POST /v1/logs` handler: `405` on non-POST, read `Content-Type`/`Content-Encoding`, call the `Forwarder`, write back Loki's status and body. Auth is intentionally absent here — task 07 adds Bearer middleware in front without touching this handler. Guard: do not inspect or mutate the OTLP payload. Spec: `.ai-factory/notes/04-write-logs-route.md`.

### Phase 3 — Token store

- [ ] **SQLite store init** — No persistence yet. Add `internal/store` with `Open(path)` using pure-Go `modernc.org/sqlite` (`sql.Open("sqlite", …)`, the only new dependency, keeps the binary cgo-free), wrapping `*sql.DB`. Create-if-not-exists `tokens(id TEXT PK, name TEXT NOT NULL, token TEXT NOT NULL UNIQUE, active INTEGER NOT NULL DEFAULT 1, created_at INTEGER NOT NULL)`; set `journal_mode=WAL` and a `busy_timeout`. `active` backs deactivation — rows are never deleted. Spec: `.ai-factory/notes/05-sqlite-store-init.md`.
- [ ] **Token operations** — Build the store API on `*Store`: `Create(name)` mints `id` + `token`=`otlp_`+base64url(32 `crypto/rand` bytes), inserts `active=1`, returns the row; `List()` returns all rows incl. plaintext `token` and `active`; `SetActive(id, bool)` updates the flag (the deactivate/reactivate path — there is **no delete**); `Lookup(token)` returns valid only when the row exists **and** `active=1`. Guard: `Lookup` must filter on `active`; unique constraint on `token`. Spec: `.ai-factory/notes/06-token-operations.md`.

### Phase 4 — Write authentication

- [ ] **Bearer auth on /v1/logs** — `/v1/logs` is currently open. Add Bearer middleware in front of the task-04 handler: parse `Authorization: Bearer <token>` (case-insensitive scheme), `401` on missing/malformed; `store.Lookup` per request — unknown or inactive → `401`, store error → `500`, valid → forward. Per-request lookup with no cache is what makes deactivation effective on the very next write. Guard: same `401` for unknown vs inactive (no oracle); never log the token. Spec: `.ai-factory/notes/07-write-bearer-auth.md`.

### Phase 5 — Grafana-authenticated admin API

- [ ] **Grafana validation client** — Add `internal/grafana` with `Client{baseURL, http}` and `Validate(ctx, token)`: GET `<grafana>/api/user` with the operator's `Authorization: Bearer`; `200`→valid, `401`/`403`→invalid, other status or transport error→`error` (caller maps to 5xx, never a silent false). Policy: any valid Grafana token is accepted — no role gate. Guard: admin-plane identity only; it never authenticates a write; never log the operator token. Spec: `.ai-factory/notes/08-grafana-validation-client.md`.
- [ ] **Admin auth middleware** — Gate the `/api/tokens` endpoints with middleware in `internal/admin`: extract the operator's `Authorization: Bearer <grafana-token>` (`401` if missing/malformed), call `grafana.Client.Validate` **per request** (no proxy session/cookie); false→`401`, Grafana-unreachable→`502`/`503`. Guard: the static GUI page `/` stays public (it only collects the token); only the API calls are gated; the two planes never mix. Spec: `.ai-factory/notes/09-admin-auth-middleware.md`.
- [ ] **Management endpoints** — Add the JSON API in `internal/admin` behind the task-09 gate, backed by `store`: `POST /api/tokens` `{name}`→`201` `{id,name,token,active,created_at}` (plaintext token returned to copy once); `GET /api/tokens`→list incl. plaintext `token` and `active`; `PATCH /api/tokens/<id>` `{active}`→`200` (the only revoke path). Under the Go 1.19 mux (no path wildcards) register `/api/tokens` (POST/GET) plus `/api/tokens/` (parse trailing id, PATCH), no router dependency. Guard: **no DELETE handler** (→`405`); bad JSON→`400`; unknown id→`404`. Spec: `.ai-factory/notes/10-token-management-endpoints.md`.

### Phase 6 — Admin GUI

- [ ] **Static asset serving** — Embed `web/` into the binary with `//go:embed web` (`embed.FS`) so the single static binary carries the GUI; serve it at `/` via `http.FileServer(http.FS(sub))` (`fs.Sub` rooted at `web/`), wired in `internal/admin`. The page is public — it only collects the operator's Grafana token client-side. Guard: `/` must not shadow `/v1/logs`, `/api/tokens`, `/healthz` (registration order); no dependency on the working directory at runtime. Spec: `.ai-factory/notes/11-static-asset-serving.md`.
- [ ] **Token management page** — Add `web/index.html` + vanilla `web/app.js` (no framework, no build). One input for the operator's Grafana token, held client-side and sent as Bearer on every `/api/tokens` call. Render `GET /api/tokens` rows showing name, plaintext token (copyable), created, and a **live `active` checkbox** whose `onchange` fires `PATCH /api/tokens/<id>` immediately — **no save buttons**; the only action button is "Create" (`POST`). Guard: handle `401` (re-enter token), revert the checkbox on a failed toggle, never persist the Grafana token. Spec: `.ai-factory/notes/12-token-management-page.md`.

### Phase 7 — Packaging & native run

- [ ] **Build automation** — Add a root `Makefile`: `build` = `CGO_ENABLED=0 go build -ldflags '-s -w' -o bin/proxy ./cmd/proxy` (static, cgo-free, embeds `web/`); plus `run`, `test` (`go test ./...`), `fmt` (`gofmt` + `go vet`), `tidy`. The `CGO_ENABLED=0` invariant ties to the pure-Go SQLite driver — no C toolchain required. Guard: `bin/` stays gitignored. Spec: `.ai-factory/notes/13-build-automation.md`.
- [ ] **Local run docs** — Document native local run (no Docker on the dev machine) in `README.md`: the four env vars (`PROXY_LISTEN`, `LOKI_URL`, `GRAFANA_URL`, `DB_PATH`) with defaults, and how a consuming SDK is pointed at the proxy — `init` `headers` = `Authorization: Bearer <token>`, OTLP endpoint = the proxy's `/v1/logs`. House doc style: behavior, prose, no trees. Out of scope: Docker image, e2e. Spec: `.ai-factory/notes/14-local-run-docs.md`.

### Phase 8 — Server deployment

- [ ] **Container image** — Add a multi-stage `Dockerfile`: builder `golang:1.19` with `CGO_ENABLED=0 go build` (embedded assets ship in the binary, no asset COPY); final `gcr.io/distroless/static` (or scratch) with just the binary, non-root, `EXPOSE 4318`, env-driven config; `DB_PATH` on a mounted volume so the token store persists. Guard: produces ONLY the image — it does **not** edit the root repo's `backend/docker-compose.yml`; compose wiring is owned independently by the root repo. Spec: `.ai-factory/notes/15-container-image.md`.

### Phase 9 — End-to-end verification

- [ ] **E2E verify** — Add `verify.sh` exercising the full path against a running proxy + Loki (+ Grafana): mint a token (`POST /api/tokens` with a Grafana Bearer), write OTLP through `POST /v1/logs` with it → expect 2xx and confirm the line via a Loki `query_range`; a bogus token → `401`; then `PATCH …{active:false}` and re-write → `401` (proves deactivation takes effect on the next write, no restart). Endpoints overridable via env; document expected status codes. Spec: `.ai-factory/notes/16-e2e-verification.md`.
