# Plan: Local run docs

## Context
Document the native (no-Docker) local run of the proxy in `README.md`: how to build and start the single binary, the four configuration settings with their defaults, and how a consuming `observe-*` SDK is pointed at the proxy with a Bearer write token.

## Settings
- Testing: no
- Logging: minimal
- Docs: no

## Tasks

### Phase 1: README run & config docs

- [x] **Task 1: Expand the "Running it" section for native local run**
  Files: `README.md`
  Replace the current single-paragraph "Running it" section with a lean prose expansion covering the native dev-machine path. State that the proxy is a single static binary: build it with `make build`, then run `./bin/proxy` (or `make run` for a source run). Note it is the only process to start beyond the Loki and Grafana the stack already runs locally — no Docker on the development machine. Add one summary sentence describing the default end state: with everything at defaults it listens on `:4318`, forwards to a local Loki at `:3100`, validates admin tokens against a local Grafana at `:3000`, and keeps its token store in `./proxy.db` in the working directory. Keep the existing note that on the server the same binary runs as a container wired by the stack's compose file. Match the existing README tone; prose, no directory trees, English.

- [x] **Task 2: Add a "Configuration" section for the four settings** (depends on Task 1)
  Files: `README.md`
  Add a new "Configuration" section after "Running it". Describe each of the four settings in prose (a sentence each), naming the env var, what it controls, and its default:
  - `PROXY_LISTEN` — the address the proxy binds; default `:4318`.
  - `LOKI_URL` — the Loki **base** URL; the proxy appends `/otlp/v1/logs` to it when forwarding writes, so it must not include that path; default `http://localhost:3100`.
  - `GRAFANA_URL` — the Grafana base URL used to validate admin (Grafana) tokens; default `http://localhost:3000`.
  - `DB_PATH` — filesystem path to the SQLite token store; default `./proxy.db`.
  Add one closing sentence noting each setting also has a command-line flag fallback (`-listen`, `-loki-url`, `-grafana-url`, `-db-path`) and that a command-line flag overrides the environment variable, which overrides the default. No flag table, prose only.

- [x] **Task 3: Add a "Pointing an SDK at the proxy" section** (depends on Task 2)
  Files: `README.md`
  Add a section explaining that a consuming `observe-*` SDK reaches the proxy exactly as it would reach Loki's OTLP endpoint, via two `init`-time settings: set the OTLP logs endpoint to the proxy's `/v1/logs` (e.g. `http://localhost:4318/v1/logs` locally), and pass a write token through the SDK's `init` `headers` as `Authorization: Bearer <token>`. Make clear this Bearer token is the proxy's **own** write token, checked per request against its store — not a Grafana token; the SDK never talks to Grafana. Note the write endpoint is the proxy's `/v1/logs`, not Loki's `/otlp/v1/logs` — the proxy forwards internally to the latter. Add a one-line forward-reference that the token is minted in the admin GUI (do not document minting here). State that an unknown or deactivated token gets a `401`, so a misconfigured SDK fails loudly rather than silently dropping logs. Use a `<token>` placeholder only — never a real token value.
