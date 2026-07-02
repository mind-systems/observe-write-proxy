# Plan: Container image

## Context
Add a multi-stage `Dockerfile` (plus a `.dockerignore`) that builds the proxy into a minimal, non-root, distroless container carrying only the static cgo-free binary with the embedded `web/` GUI — configured entirely by the four canon env vars, with `DB_PATH` intended for a mounted volume. Produces only the image; no compose wiring.

## Settings
- Testing: no
- Logging: minimal
- Docs: no

## Tasks

### Phase 1: Build context hygiene

- [x] **Task 1: Add `.dockerignore`**
  Files: `.dockerignore`
  Create a `.dockerignore` at the repo root excluding artifacts that must never enter the build context: `.git`, `.ai-factory`, `bin/`, local SQLite state (`*.db`, `*.db-shm`, `*.db-wal`), `*.log`, `.env`/`.env.*`, and editor/OS cruft (`.DS_Store`, `.idea/`, `.vscode/`). Do NOT exclude `web/` — the GUI assets are compiled into the binary via `//go:embed web` (see `web_embed.go`) and must be present at build time. Mirror the intent of the existing `.gitignore` so a developer's local `proxy.db` or `.git` never bloats the context or leaks tokens into a layer.

### Phase 2: Container image

- [x] **Task 2: Add multi-stage `Dockerfile`** (depends on Task 1)
  Files: `Dockerfile`
  Create a multi-stage `Dockerfile` at the repo root:
  - **Builder stage** `FROM golang:1.19 AS build`: set `WORKDIR /src`; copy `go.mod`/`go.sum` and run `go mod download` as a separate cached layer *before* copying the rest of the source; then copy the full source and build with `RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /proxy ./cmd/proxy`. `CGO_ENABLED=0` is mandatory (ties to the pure-Go `modernc.org/sqlite` driver and the static-binary invariant already in the `Makefile`). No asset COPY — `web/` ships inside the binary via the module-root `//go:embed web`.
  - **Final stage** `FROM gcr.io/distroless/static:nonroot AS final`: `COPY --from=build /proxy /proxy` and nothing else (no `web/`, no tools, no shell). Set `USER nonroot:nonroot` (uid 65532), `EXPOSE 4318` (documentation only — the canon `PROXY_LISTEN` default), and `ENTRYPOINT ["/proxy"]` with no `CMD` args (all config comes from env via `internal/config`, note 01).
  - Add a short comment noting runtime config is env-driven (`PROXY_LISTEN`, `LOKI_URL`, `GRAFANA_URL`, `DB_PATH`), that `LOKI_URL`/`GRAFANA_URL` must be supplied at deploy time (the `localhost` canon defaults are wrong inside a container), and that `DB_PATH` must point inside a mounted volume writable by uid 65532 so the token store persists across container replacement.
  - **Do NOT** add a Dockerfile `HEALTHCHECK` (distroless has no shell/`curl`; liveness is an orchestration concern). **Do NOT** touch or reference the root repo's `backend/docker-compose.yml` — compose wiring is owned independently by the root repo. This task produces only the image; the proxy repo stays self-contained.
