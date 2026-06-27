# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A thin **Go service that guards the OTLP log write path** in front of Loki. SDKs no longer post logs straight to Loki; they post to this proxy, which authenticates each write against its own token store and forwards only valid traffic on to Loki. It exists so the log-ingestion endpoint can be exposed on a network (a shared server, reachable by mobile and browser clients) without becoming an open, unauthenticated log sink.

It is infrastructure, not an application: no business logic, no knowledge of any consuming project, no log content awareness beyond pass-through. One process serves both the write path and a small admin surface; they are separated by URL path, not by host or port.

## Two auth planes — the spine

The whole design rests on keeping these apart. They never mix.

- **Write plane** (SDK → proxy → Loki). Authenticated by the proxy's **own** tokens, kept in its own store. A write carries `Authorization: Bearer <proxy-token>`; the proxy checks the token against its allow-list and either forwards the request to Loki or rejects it with `401`. Grafana is not involved in this path — there is no per-write round-trip to anything. Deactivation is immediate: a token switched off fails the very next write. Tokens are never deleted — only flipped inactive — so a token can be switched back on.

- **Admin plane** (a human → the proxy's GUI and management API). Authenticated by a **Grafana** token. Grafana is the identity provider for "who may mint and revoke write tokens" — this proxy has no user accounts and no registration of its own. The admin presents their Grafana token once in the GUI; every subsequent management request is validated by calling Grafana. Anyone holding a valid Grafana token may manage write tokens.

The reason for the split: write tokens must be fast to check and instantly revocable, so they are local and owned here; "who is allowed to administer" is real user identity, which Grafana already solves, so it is delegated. A Grafana token never authenticates a write, and a write token never grants admin access.

## The write path

`POST /v1/logs` with `Authorization: Bearer <proxy-token>`. The proxy looks the token up in its store; on a hit it forwards the body unchanged to Loki's OTLP endpoint (`http://localhost:3100/otlp/v1/logs` locally), on a miss it returns `401`.

The proxy is **label-blind**: it does not read, add, or rewrite any OTLP attributes or Loki labels. The frozen label policy (`project`, `service_name`, `level` as the only index labels; everything else structured metadata) is owned upstream by the SDKs and downstream by Loki — the proxy only authenticates and forwards.

## The admin plane

A minimal GUI served by the proxy itself: a single field for a Grafana token. Once entered, the token is held client-side and sent with each management call; the proxy validates it against Grafana (a lightweight authenticated Grafana API call — valid means allowed) on **every** request rather than minting a session of its own. No role gate beyond "the Grafana token is valid".

Through the GUI an operator creates, lists, and toggles write tokens between active and inactive. Creation takes a **name** and returns a freshly generated token. Switching a token off deactivates it — taking effect on the next write with no restart and no reload — and the token is never deleted, so it can be switched back on. The toggle is a live control: flipping it sends the change immediately, with no save step.

## Token store

A local **SQLite** database, accessed through a pure-Go driver so the service stays a single static binary with no cgo. It holds, per write token, a **name**, an **active flag**, and the **token value stored in plaintext** (so the value can be read back later in the GUI — chosen for convenience on a self-hosted, single-operator stack). Revoking a token sets its active flag to false rather than deleting the row, so history is kept and the token can be reactivated.

The **name is a display-only label**. It exists solely so the operator can recognise which token to revoke when they no longer want logs from some source. It is never sent anywhere, never matched against request payloads, and carries no authorization meaning — a write token's name does not constrain what it may write.

## Routing — one host, paths differ

Everything is served from one address; the path selects the plane:

- the write path (`/v1/logs`) — write-token auth;
- the admin GUI and its management API — Grafana-token auth.

Distinguishing the planes by path, not by port, is deliberate: a single exposed endpoint, a single thing to put behind a reverse proxy on the server.

## Distribution & deployment

Unlike the `observe-*` SDKs, this proxy is **not pinned by consumers via a git URL** — nothing depends on it as a library. It is a **deployed service**:

- **Local (macOS):** runs as a native single binary — no Docker on the developer machine, per the stack's hard constraint.
- **Server:** the same binary runs as a container, added as a service alongside Loki and Grafana in the root observability repo's `backend/docker-compose.yml`. That compose file — the cross-service wiring — lives in the **root** coordinator repo, not here.

## Relationship to the wider system

This is one sub-repo of the local observability stack. The **root** `observability` repo is the coordinator (architecture, roadmap, the shared compose file); the `observe-swift` / `observe-dart` / `observe-js` SDKs produce the OTLP traffic this proxy guards and already carry the `Authorization: Bearer …` credential through their `init` `headers` parameter. The read path is separate and does not pass through here: the `observe-logs` skill queries Loki directly and never writes.

## Language

All files — code, docs, config — are written in **English**, regardless of the conversation language.
