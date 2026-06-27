# 14 — Local run docs

**Task:** ROADMAP → Phase 7 — Packaging & native run → "Local run docs"
**Depth:** low

## Goal

A `README.md` section that tells an operator how to run the proxy natively on the dev machine — no Docker, per the stack's hard constraint — and how to point a consuming SDK at it. Someone with the binary should be able to start the proxy, understand the four settings and their defaults, and wire an `observe-*` SDK through it with a Bearer token, without reading the source.

## Design

- Edit `README.md` at the repo root, adding (or filling out) a "Running it" section and a "Configuration" section. Prose, not tables-of-methods; no directory trees; English; lean. Describe behavior, not code.
- **Running it** — explain that the proxy is a single native binary: build with `make build`, then run `./bin/proxy` (or `make run` for a source run). It is the only process to start beyond the Loki and Grafana the stack already runs locally; no Docker on the dev machine. State the obvious default end state in one line: with everything at defaults it listens on `:4318`, forwards to a local Loki at `:3100`, validates admin tokens against a local Grafana at `:3030`, and keeps its token store in `./proxy.db` in the working directory.
- **Configuration** — describe the four settings, each as a sentence naming the env var, what it controls, and its default. Keep it prose; the four are:
  - `PROXY_LISTEN` — the address the proxy binds; default `:4318`.
  - `LOKI_URL` — the Loki **base** URL; the proxy forwards writes to that base plus `/otlp/v1/logs`; default `http://localhost:3100`.
  - `GRAFANA_URL` — the Grafana base URL used to validate admin (Grafana) tokens; default `http://localhost:3030`.
  - `DB_PATH` — filesystem path to the SQLite token store; default `./proxy.db`.
  - Mention that each also has a command-line flag fallback (`-listen`, `-loki-url`, `-grafana-url`, `-db-path`) and that a flag passed on the command line overrides the environment, which overrides the default — one sentence, no flag table.
- **Pointing an SDK at the proxy** — explain that a consuming `observe-*` SDK reaches the proxy exactly as it would reach Loki's OTLP endpoint, with two `init`-time settings: set the OTLP logs endpoint to the proxy's `/v1/logs` (e.g. `http://localhost:4318/v1/logs` locally), and pass a write token through the SDK's `init` `headers` as `Authorization: Bearer <token>`. The token is one minted in the admin GUI (forward-reference the admin plane / GUI, do not document minting here). Make the credential's role clear: this Bearer token is the proxy's **own** write token, checked per request against its store — it is not a Grafana token, and the SDK never talks to Grafana. State that an unknown or deactivated token gets a `401` so a misconfigured SDK fails loudly rather than silently dropping logs.

## Edge cases / watch

- Do not document the admin GUI workflow, token minting, or Grafana admin auth here beyond a one-line pointer — that surface belongs to the admin/GUI tasks; this note is strictly the native run path and the write-side wiring.
- Keep `LOKI_URL` described as the **base** URL, mirroring the config note: the `/otlp/v1/logs` suffix is appended by the proxy, so an operator setting it must not include that path. Calling this out prevents the double-path mistake.
- The write endpoint the SDK targets is the proxy's `/v1/logs`, not Loki's `/otlp/v1/logs` — be precise, because the two paths look similar and only the proxy's is authenticated. The SDK points at `<proxy>/v1/logs`; the proxy internally forwards to `<loki>/otlp/v1/logs`.
- Per the never-leak rule, the README must not embed a real token value in examples — use a `<token>` placeholder.
- Match the existing README's tone and any structure already there; do not restructure the file, just add the two sections lean.

## Out of scope

- Docker / server container run and the `docker-compose.yml` wiring — task 15.
- End-to-end verification that a real SDK write lands in Loki — task 16.
- The `Makefile` targets themselves — task 13 (this note only references `make build` / `make run` as the entry points).
- Admin GUI usage and token minting documentation.

## Done when

- `README.md` has a native local-run section: build with `make build`, run the single binary, no Docker.
- The four settings — `PROXY_LISTEN`, `LOKI_URL`, `GRAFANA_URL`, `DB_PATH` — are each documented in prose with their canon defaults (`:4318`, `http://localhost:3100`, `http://localhost:3030`, `./proxy.db`).
- The README shows how a consuming SDK is pointed at the proxy: OTLP endpoint at the proxy's `/v1/logs` plus an `Authorization: Bearer <token>` header carrying a proxy write token, with the `401`-on-bad-token behavior noted.
