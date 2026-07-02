# observe-write-proxy

A thin Go service that guards the OTLP log write path in front of Loki. SDKs send their logs here instead of straight to Loki; the proxy authenticates each write and forwards only valid traffic on. It exists so the log-ingestion endpoint can be exposed on a network — reachable by mobile and browser clients — without becoming an open, unauthenticated log sink.

Part of the local observability stack, a sibling of the `observe-*` SDKs — but a deployed service rather than a library anyone pins.

## How it works

The proxy keeps two authentication planes strictly apart.

**Writing logs.** An SDK posts OTLP/HTTP to the proxy with its own proxy-issued token in the `Authorization` header. The proxy checks the token against its local store and, on a match, forwards the request body **unchanged** to Loki — it never reads, adds, or rewrites any labels. An unknown token is rejected. Revoking a token takes effect on the very next write: no restart, no reload.

**Managing tokens.** A small admin GUI lets an operator mint, list, and revoke write tokens. Access to it is gated by a Grafana token — Grafana is the identity provider, so the proxy has no accounts or sign-up of its own. Each token is stored with a human-readable name so the operator can tell at a glance which one to revoke. A Grafana token never authorizes a write, and a write token never reaches the admin plane.

Both planes are served from one address, separated by URL path.

## Running it

The proxy is a single static binary. Build it with `make build`, which produces `./bin/proxy`, then start it with `./bin/proxy` — or skip the build step and run it straight from source with `make run`. Locally it is the only process to start beyond the Loki and Grafana the stack already runs: no Docker required on the development machine. With everything left at its defaults it listens on `:4318`, forwards writes to a local Loki at `:3100`, validates admin tokens against a local Grafana at `:3000`, and keeps its token store in `./proxy.db` in the working directory. On the server the same binary runs as a container alongside Loki and Grafana, wired up by the observability stack's compose file.

## Configuration

Four settings control the proxy, each an environment variable with a canon default.

- `PROXY_LISTEN` — the address the proxy binds for both the write and admin planes; defaults to `:4318`.
- `LOKI_URL` — Loki's **base** URL; the proxy appends `/otlp/v1/logs` to it when forwarding writes, so this must not already include that path; defaults to `http://localhost:3100`.
- `GRAFANA_URL` — the base URL of the Grafana instance used to validate admin (Grafana) tokens; defaults to `http://localhost:3000`.
- `DB_PATH` — the filesystem path to the SQLite token store; defaults to `./proxy.db`.

Each setting also has a command-line flag fallback (`-listen`, `-loki-url`, `-grafana-url`, `-db-path`); a flag overrides the environment variable, which overrides the default.

## Pointing an SDK at the proxy

A consuming `observe-*` SDK reaches the proxy exactly as it would reach Loki's OTLP endpoint directly, via two `init`-time settings: point the OTLP logs endpoint at the proxy's `/v1/logs` (e.g. `http://localhost:4318/v1/logs` locally, not Loki's `/otlp/v1/logs` — the proxy forwards internally to the latter), and pass a write token through the SDK's `init` `headers` as `Authorization: Bearer <token>`. This Bearer token is the proxy's **own** write token, checked per request against its local store — it is not a Grafana token, and the SDK never talks to Grafana. The token itself is minted in the admin GUI (minting is not covered here). An unknown or deactivated token gets a `401`, so a misconfigured SDK fails loudly rather than silently dropping logs.

## End-to-end verification

`verify.sh` at the repo root exercises the full write path against a running proxy, Loki, and Grafana — the same stack `make run` (or the deployed binary) talks to. It is complementary to the root repo's `../backend/verify.sh` (`make backend-verify`), which ingests straight into Loki and never touches the proxy; this script instead proves the proxy's two auth planes actually gate access.

Export `GRAFANA_TOKEN` — a Grafana user/session-style token, since the proxy validates it via `GET /api/user` — then run the script. `PROXY_URL` and `LOKI_URL` are optional overrides; both default to the local canon addresses (`:4318` and `:3100`).

```
GRAFANA_TOKEN=<grafana-user-token> ./verify.sh
```

It walks through four steps and exits non-zero on the first failure, so it doubles as a smoke gate:

| Step | Call | Expected |
|---|---|---|
| 1 | `POST /api/tokens` with the Grafana token | `201`, a fresh write token minted |
| 2 | `POST /v1/logs` with the write token, then read back via Loki's `query_range` | `2xx`, and the written marker is found |
| 3 | `POST /v1/logs` with a bogus token | `401` |
| 4 | `PATCH /api/tokens/<id>` to deactivate, then the same write token rewritten | `2xx`, then `401` — proving deactivation takes effect on the very next write, with no restart and no reload |

## More

`CLAUDE.md` is the source of truth for the contract and design — the two-plane model, the write path, the token store, and the deployment shape.
