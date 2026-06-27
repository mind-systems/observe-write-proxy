# observe-write-proxy

## Overview

A thin Go service that guards the OTLP log write path in front of Loki. SDKs post their logs to this proxy instead of directly to Loki; the proxy authenticates each write against its own token store and forwards only valid traffic on. It exists so the log-ingestion endpoint can be exposed on a network — reachable by mobile and browser clients — without becoming an open, unauthenticated log sink. Part of the local observability stack; a sibling of the `observe-*` SDKs, but a deployed service rather than a pinned library.

## Core Features

- Write-path authentication: `POST /v1/logs` with `Authorization: Bearer <proxy-token>` checked against a local allow-list; valid requests forwarded to Loki's OTLP endpoint, invalid ones rejected with `401`
- Label-blind forwarding: the proxy never reads, adds, or rewrites OTLP attributes or Loki labels — it only authenticates and proxies
- Own token store: proxy-issued tokens kept in SQLite; deactivation is immediate (a token switched off fails the next write — no restart, no reload — and is never deleted, so it can be reactivated)
- Grafana-authenticated admin plane: a minimal GUI and management API for minting, listing, and toggling write tokens active or inactive, gated by a valid Grafana token (Grafana is the identity provider — the proxy has no user accounts of its own)
- Single host, planes split by URL path: write path under `/v1/logs`, admin GUI and management API under their own paths, one address behind one reverse proxy

## Tech Stack

- **Language:** Go (single static binary, no cgo)
- **Token store:** SQLite via a pure-Go driver (e.g. `modernc.org/sqlite`)
- **Wire protocol:** OTLP/HTTP (pass-through to Loki `…/otlp/v1/logs`)
- **Admin identity:** Grafana token validated per request against the Grafana API
- **Distribution:** deployed service — native binary locally (no Docker on the dev machine), the same binary as a container in the root repo's `backend/docker-compose.yml`; not pinned by consumers

## Two Auth Planes

- **Write plane** (SDK → proxy → Loki): proxy-owned tokens, SQLite-backed, instant revoke. Grafana is not on this path — no per-write round-trip.
- **Admin plane** (operator → GUI / management API): Grafana token validated per request. No registration, no session of the proxy's own.

A Grafana token never authenticates a write; a write token never grants admin access.

## Token Store

Per write token: a **name** (display-only label, never matched against payloads, no authorization meaning), an **active flag**, and the **token value in plaintext** (readable back in the GUI — chosen for convenience on a self-hosted, single-operator stack). The name exists only so the operator recognises which token to deactivate. Revoking sets `active` to false rather than deleting the row, so tokens are kept and can be reactivated.

## Non-Functional Requirements

- **Never break ingestion semantics:** the proxy forwards bodies unchanged; the frozen Loki label policy (`project`, `service_name`, `level` as the only index labels) is owned upstream by the SDKs and downstream by Loki.
- **Immediate (de)activation:** flipping a token's active flag takes effect on the next request without restart or reload; tokens are deactivated, never deleted.
- **Single binary:** no cgo, no external runtime dependencies, so the same artifact runs natively on macOS and as a minimal container on the server.

## Architecture

See `.ai-factory/ARCHITECTURE.md` for module boundaries and dependency rules.
