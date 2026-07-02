# Code Review: 15 — Container image

**Plan:** `.ai-factory/plans/15-container-image.md`
**Spec note:** `.ai-factory/notes/15-container-image.md`
**Changed files reviewed:** `Dockerfile`, `.dockerignore` (the only application-affecting changes; the plan/plan-review/json artifacts are non-code).
**Cross-checked against:** `go.mod`, `Makefile`, `web_embed.go`, `internal/config/config.go`, `cmd/proxy/main.go`, `.gitignore`.

## Scope

The milestone adds only a build/packaging surface — no Go code, no handlers, no store changes. There is no runtime code path to break: no migrations, no type changes, no concurrency. The correctness question is entirely "does this Dockerfile produce a working image, and does `.dockerignore` keep the build correct and clean."

## Verification performed

- **Build reproduces.** Ran the exact builder command locally: `CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /proxy ./cmd/proxy` → succeeds, produces a ~9 MB self-contained binary. No cgo, matching the pure-Go `modernc.org/sqlite` invariant.
- **Embed reachable, so "no asset COPY" is safe.** `web/` contains `app.js` + `index.html`; `web_embed.go` (`//go:embed web`, package at module root) is imported transitively from `cmd/proxy`, so the GUI is compiled into the binary. The final stage copying only `/proxy` genuinely ships the GUI. `.dockerignore` correctly does **not** exclude `web/` (needed at build time) — confirmed with an explicit comment.
- **Go version match.** Builder `golang:1.19` == `go 1.19` in `go.mod`.
- **Env contract match.** `PROXY_LISTEN` / `LOKI_URL` / `GRAFANA_URL` / `DB_PATH` and the `:4318` default line up with `internal/config/config.go`. `ENTRYPOINT ["/proxy"]` with no `CMD` is correct — `config.Load()` reads env with flag fallback, no baked args needed. `EXPOSE 4318` matches the listen default (documentation only).
- **Non-root sound.** `distroless/static:nonroot` + `USER nonroot:nonroot` (uid 65532) binding unprivileged `:4318` needs no root. `.dockerignore` mirrors `.gitignore` intent (`.git`, `.ai-factory`, `bin/`, `*.db*`, `.env*`, editor cruft), preventing a local `proxy.db` from leaking tokens into a layer.
- **Guards honored.** No Dockerfile `HEALTHCHECK` (correct — no shell/curl in distroless), no `backend/docker-compose.yml` edits, no registry/CI scope creep. Matches the roadmap guard and the spec's out-of-scope list.

## Findings

None. No correctness, security, or runtime-breakage defects.

The following are **non-blocking observations only**, requiring no change:

- **cwd `/` is not writable by uid 65532 (no `WORKDIR` in final stage).** If an operator forgets `DB_PATH`, the `./proxy.db` default resolves to `/proxy.db` and `store.Open` fails at startup, crash-looping the container. This is desirable fail-fast behavior (loud, immediate) rather than silently persisting to the ephemeral layer, and the plan mandates `DB_PATH` on a mounted volume. Intentional-friendly, not a bug.
- **`/tmp` dependency of the SQLite driver.** `modernc.org/sqlite` may touch `os.TempDir()`; `gcr.io/distroless/static:nonroot` ships a `/tmp`, so this works out of the box. Worth remembering only if the base is ever swapped for bare `scratch` (no `/tmp`) — but the plan chose distroless precisely for such niceties.

REVIEW_PASS
