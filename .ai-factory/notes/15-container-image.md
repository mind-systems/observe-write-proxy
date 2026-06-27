# 15 — Container image

**Task:** ROADMAP → Phase 8 — Server deployment → "Container image"
**Depth:** medium

## Goal

A multi-stage `Dockerfile` at the repo root produces a minimal, self-contained container image that runs the proxy entirely from environment configuration. The image carries only the static, cgo-free binary — the admin GUI is already embedded in it — so it has no shell, no package manager, and nothing on disk beyond the executable. On the server the token store lives on a mounted volume so it survives container replacement.

## Design

- **Builder stage** (`FROM golang:1.19 AS build`): copy the module in, run `CGO_ENABLED=0 go build -o /proxy ./cmd/proxy`. `CGO_ENABLED=0` is mandatory, not incidental — it produces a fully static binary that runs on a distroless/scratch base, and it is consistent with the canon's pure-Go `modernc.org/sqlite` driver (the whole point of choosing that driver was to keep cgo off). Build with `GOOS=linux` so the artifact targets the server regardless of the host that builds it. **Decision:** do the `go mod download` as a separate layer before copying the full source, so dependency fetches are cached across source-only changes.
- **No asset COPY in the final stage.** The `web/` GUI is compiled into the binary via the `//go:embed web` directive (note 11), so the assets travel inside `/proxy`. The final stage copies exactly one file — the binary — and nothing from `web/`. Copying `web/` into the final image would be dead weight that is never read.
- **Final stage** (`FROM gcr.io/distroless/static AS final`): `COPY --from=build /proxy /proxy` and nothing else. `distroless/static` is the right base for a static cgo-free binary — it has no libc dependency to satisfy and ships CA certificates plus `/etc/passwd` for the nonroot user. **Decision:** prefer `gcr.io/distroless/static` over bare `scratch`. `scratch` would also work for the pure binary, but distroless gives a ready nonroot user and CA roots without hand-assembling them; the proxy's outbound calls (forwarding to Loki, validating against Grafana) are plain HTTP to in-cluster service names, but the CA roots cost nothing and keep an HTTPS Grafana/Loki option open.
- **Run as non-root.** Use the distroless nonroot variant tag (`gcr.io/distroless/static:nonroot`) and `USER nonroot:nonroot` (uid 65532). The proxy binds `:4318` (unprivileged) and needs no root, so dropping privileges has no cost. The mounted volume holding the SQLite file must therefore be writable by that uid.
- **`EXPOSE 4318`** — the canon default listen port (`PROXY_LISTEN` default `:4318`). `EXPOSE` is documentation only; the actual bind is still driven by config.
- **`ENTRYPOINT ["/proxy"]`**, no `CMD` arguments. The proxy takes all its configuration from the four canon env vars (which `internal/config` already reads, with flag fallback — note 01), so the entrypoint needs no baked-in flags.
- **Configuration is purely environmental at runtime.** The four canon settings are passed as env at `docker run` / compose time:
  - `PROXY_LISTEN` — listen address, default `:4318`.
  - `LOKI_URL` — on the server this points at the **compose service name** for Loki (e.g. `http://loki:3100`), not `localhost`. The container's `localhost` is the container, so the local default is wrong inside the image; the value must be supplied at deploy time.
  - `GRAFANA_URL` — likewise the **compose service name** for Grafana (e.g. `http://grafana:3000`), supplied at deploy time.
  - `DB_PATH` — must point **inside a mounted volume** (e.g. `/data/proxy.db`) so the token store persists across container restarts and image upgrades. A `DB_PATH` left at the `./proxy.db` default would write into the container's ephemeral layer and lose every minted token when the container is replaced. The image documents the intended data directory but does not own the volume wiring.
- The `Dockerfile` lives at the proxy repo root; add a `.dockerignore` excluding `.git`, `.ai-factory`, and any local `*.db` so a developer's local `proxy.db` is never baked into the build context.

## Edge cases / watch

- **No shell in distroless** → a shell-form `HEALTHCHECK` (`HEALTHCHECK CMD curl ...`) cannot run; there is no `curl`, no `/bin/sh`. Do **not** add a Dockerfile `HEALTHCHECK` here. Liveness against `/healthz` is an orchestration concern (compose/k8s probe using an exec or HTTP probe), owned by whoever wires the service — out of scope for the image. Leaving a broken healthcheck in the image is worse than none.
- **Volume permissions vs non-root**: because the container runs as uid 65532, the host/volume backing `DB_PATH`'s directory must be writable by that uid, or SQLite open fails at startup. This is a deploy-time concern but must be flagged so the volume is provisioned writable, not root-owned.
- **`localhost` defaults are a trap inside a container.** The canon defaults (`http://localhost:3100`, `http://localhost:3030`) are correct for the native macOS run but wrong inside the image, where `localhost` is the container itself. The image must rely on `LOKI_URL`/`GRAFANA_URL` being supplied; do not bake server hostnames into the image.
- **Build context size**: without `.dockerignore`, the `web/` assets are fine (they belong in the binary), but a local `proxy.db` or `.git` would bloat the context and could leak local tokens into a build layer. Exclude them explicitly.
- Keep the final image to the single binary — resist adding debugging tools "just in case"; they reintroduce a shell and attack surface the distroless base exists to remove.

## Out of scope

- **The `backend/docker-compose.yml` wiring.** This task produces only the image. Adding the proxy as a third service alongside Loki and Grafana — the service name, the env values, the volume mount, the network — is tracked in the **root** observability repo's roadmap and lives in the root coordinator repo's compose file. This note makes **no** edits and **no** cross-references to that file; the proxy repo stays self-contained.
- Any liveness/readiness probe configuration (an orchestration concern, see the healthcheck note above).
- Pushing/tagging the image to a registry, CI build wiring, and image versioning policy.
- The native macOS run path (plain binary, no container) — unchanged by this task.

## Done when

- `docker build` against the proxy repo root produces a minimal image (distroless/scratch base) containing only the static cgo-free binary with the `web/` GUI embedded, no shell, and a non-root user.
- Running that image with `PROXY_LISTEN`, `LOKI_URL`, `GRAFANA_URL`, and `DB_PATH` supplied as env starts the proxy, serves `/v1/logs`, `/api/tokens`, `/`, and `/healthz`, and forwards valid writes to the `LOKI_URL` target.
- With `DB_PATH` pointed at a mounted volume, minted tokens persist across a container restart / replacement.
- The image carries no Dockerfile `HEALTHCHECK` and no compose edits ship with this task.
