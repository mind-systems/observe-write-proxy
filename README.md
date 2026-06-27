# observe-write-proxy

A thin Go service that guards the OTLP log write path in front of Loki. SDKs send their logs here instead of straight to Loki; the proxy authenticates each write and forwards only valid traffic on. It exists so the log-ingestion endpoint can be exposed on a network — reachable by mobile and browser clients — without becoming an open, unauthenticated log sink.

Part of the local observability stack, a sibling of the `observe-*` SDKs — but a deployed service rather than a library anyone pins.

## How it works

The proxy keeps two authentication planes strictly apart.

**Writing logs.** An SDK posts OTLP/HTTP to the proxy with its own proxy-issued token in the `Authorization` header. The proxy checks the token against its local store and, on a match, forwards the request body **unchanged** to Loki — it never reads, adds, or rewrites any labels. An unknown token is rejected. Revoking a token takes effect on the very next write: no restart, no reload.

**Managing tokens.** A small admin GUI lets an operator mint, list, and revoke write tokens. Access to it is gated by a Grafana token — Grafana is the identity provider, so the proxy has no accounts or sign-up of its own. Each token is stored with a human-readable name so the operator can tell at a glance which one to revoke. A Grafana token never authorizes a write, and a write token never reaches the admin plane.

Both planes are served from one address, separated by URL path.

## Running it

The proxy is a single static binary. Locally it runs as a native process — no Docker required on the development machine. On the server the same binary runs as a container alongside Loki and Grafana, wired up by the observability stack's compose file.

## More

`CLAUDE.md` is the source of truth for the contract and design — the two-plane model, the write path, the token store, and the deployment shape.
