# Plan Review: 15 — Container image

**Plan:** `.ai-factory/plans/15-container-image.md`
**Spec note:** `.ai-factory/notes/15-container-image.md`
**Files Reviewed:** plan + note + codebase (`go.mod`, `Makefile`, `web_embed.go`, `internal/admin/static.go`, `internal/config/config.go`, `cmd/proxy/main.go`, `.gitignore`, `ARCHITECTURE.md`, `rules/base.md`)
**Risk Level:** 🟢 Low

## Context Gates

- **Architecture (WARN → PASS):** `ARCHITECTURE.md` fixes `web/` at the module root and mandates a single static, cgo-free binary. The plan's reliance on `//go:embed web` (`web_embed.go`, package `webassets` at module root) and `CGO_ENABLED=0` is consistent. No boundary violation — the Dockerfile touches no application code.
- **Rules (PASS):** `rules/base.md` requires "one static binary, no cgo — keep all dependencies pure-Go." The mandatory `CGO_ENABLED=0` and pure-Go `modernc.org/sqlite` tie-in are honored. No convention conflict.
- **Roadmap (PASS):** Maps exactly to ROADMAP Phase 8 → "Container image." Scope guard ("produces ONLY the image; does not edit `backend/docker-compose.yml`") matches the roadmap item and the proxy repo's self-contained boundary. Correctly defers compose wiring to the root repo per both CLAUDE.md files.

## Verification against the codebase

Every concrete assumption in the plan checks out:

- **Env vars & defaults** — `PROXY_LISTEN` / `LOKI_URL` / `GRAFANA_URL` / `DB_PATH` with defaults `:4318`, `http://localhost:3100`, `http://localhost:3000`, `./proxy.db` match `internal/config/config.go` exactly. The "`localhost` defaults are a trap in a container" observation is correct and important.
- **Embed reachability** — the critical correctness point for "no asset COPY." `web_embed.go` (package `webassets`) is imported by `internal/admin/static.go` (`webassets "github.com/mind-systems/observe-write-proxy"`), which is reachable from `cmd/proxy`. So `go build ./cmd/proxy` *does* compile the embed and bake `web/` into the binary — the final stage genuinely needs only the binary. Confirmed, not assumed.
- **Build invariant** — `CGO_ENABLED=0 ... -ldflags "-s -w" -o ... ./cmd/proxy` mirrors the `Makefile` `build` target; adding `GOOS=linux` is correct for host-independent builds.
- **Go version** — builder `golang:1.19` matches `go 1.19` in `go.mod`.
- **Port** — `EXPOSE 4318` matches the `PROXY_LISTEN` default; correctly noted as documentation-only.
- **Non-root** — `distroless/static:nonroot` (uid 65532) binding unprivileged `:4318` is sound; `distroless/static` (not `scratch`) is the right call since it ships CA roots for an HTTPS Grafana/Loki option.
- **ENTRYPOINT, no CMD** — correct: `config.Load()` reads env with flag fallback, no baked args needed.
- **No HEALTHCHECK** — correct given no shell/curl in distroless; `/healthz` exists in `main.go` but liveness is rightly left to the orchestrator.
- **`.dockerignore`** — mirrors `.gitignore` intent (`.git`, `bin/`, `*.db*`, `.env*`, editor cruft) and explicitly preserves `web/`. No conflict with the embed.

## Observations (non-blocking, no action required)

- **Runtime `/tmp` for the SQLite driver:** `modernc.org/sqlite` may use `os.TempDir()` (`/tmp`) for scratch operations. `gcr.io/distroless/static:nonroot` ships a sticky-bit `/tmp`, so this works out of the box — flagged only so it isn't a surprise if the base image is ever swapped for bare `scratch` (which has no `/tmp`).
- **No `WORKDIR` in final stage:** cwd defaults to `/`, which uid 65532 cannot write. If an operator forgets `DB_PATH`, the `./proxy.db` default resolves to `/proxy.db` and `store.Open` fails fast at startup. This is desirable fail-fast behavior (better than silently persisting to the ephemeral layer), and the plan already mandates `DB_PATH` on a mounted volume — so no change needed, just confirming the behavior is intentional-friendly.
- **`GOOS=linux` redundant inside `golang:1.19`** (already linux) but harmless and correct for cross-build clarity.

## Positive Notes

- The embed-reachability reasoning is the one place this plan could have been wrong ("no asset COPY" would silently ship a GUI-less image if `webassets` were unreferenced); the plan got it right and the code confirms it.
- Scope discipline is excellent: no compose edits, no registry/CI/versioning scope creep, no debugging tools in the final image — each explicitly ruled out with a rationale.
- Layer-caching decision (`go mod download` before source copy) and the distroless-over-scratch tradeoff are both justified rather than asserted.
- The four-env-var runtime contract and the `localhost`-inside-container trap are called out precisely.

The plan is complete, technically accurate against the current codebase, and free of architectural or security defects.

PLAN_REVIEW_PASS
