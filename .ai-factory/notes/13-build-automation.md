# 13 — Build automation

**Task:** ROADMAP → Phase 7 — Packaging & native run → "Build automation"
**Depth:** low

## Goal

A repo-root `Makefile` that captures the few commands the project actually needs — build, run, test, fmt, tidy — so the single static no-cgo binary is produced the same way every time and a contributor never has to remember the exact `go build` flag string. The canonical artifact is one self-contained binary at `bin/proxy` with the `web/` GUI assets embedded, buildable on macOS with no C toolchain present.

## Design

- New `Makefile` at the repo root. Targets are thin wrappers over `go` commands; declare them all in `.PHONY` since none correspond to real files (except the binary, which `make` need not track — always rebuild on demand).
- `build` — `CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/proxy ./cmd/proxy`.
  - `CGO_ENABLED=0` is the load-bearing flag: it forces a pure-Go compile so the resulting binary has no dynamic libc linkage and needs no C toolchain to build or run. This is exactly what the pure-Go SQLite driver (`modernc.org/sqlite`, driver name `"sqlite"`) buys us — the moment cgo creeps in, that invariant is gone and the "single static binary" promise breaks. The flag is therefore not a nicety; it is the contract.
  - `-ldflags "-s -w"` strips the symbol table (`-s`) and DWARF debug info (`-w`), shrinking the binary. These are safe for a service binary (no debugger attach expected in normal run); they also make the output more reproducible by dropping debug metadata.
  - `web/` assets ride along automatically: the GUI is embedded via `//go:embed` in `internal/admin` (per the admin/GUI tasks), so `go build` pulls `web/` into the binary — there is no separate asset-bundling step and `bin/proxy` is genuinely self-contained.
- `run` — `go run ./cmd/proxy`. Plain run for local iteration; it inherits the env vars (`PROXY_LISTEN`, `LOKI_URL`, `GRAFANA_URL`, `DB_PATH`) from the caller's shell, so no flags are baked into the target. **Decision:** `run` uses `go run` rather than depending on the `build` target, so a quick run doesn't litter `bin/` and picks up source changes directly.
- `test` — `go test ./...`. Whole-module test sweep.
- `fmt` — run both `gofmt` and `go vet`: `gofmt -l -w .` then `go vet ./...`. `gofmt -l -w` reformats in place and lists what it touched; `go vet` is the cheap static check. **Decision:** fold vet into `fmt` rather than a separate `vet` target — the project's command surface stays minimal (canon: prefer the standard library, no extra tooling) and "tidy up before commit" is one verb.
- `tidy` — `go mod tidy`. Prunes and syncs `go.mod`/`go.sum`.

## Edge cases / watch

- `bin/` must be gitignored. Confirm the repo's `.gitignore` already excludes built binaries (a broad Go-style ignore of compiled output, or an explicit `/bin/`); if it does not, add `/bin/` — **state this in the task, do not edit files here**. The binary is a build artifact, never committed.
- `CGO_ENABLED=0` belongs **inline on the `build` recipe**, not exported globally from the shell, so the guarantee travels with the Makefile and survives a contributor whose environment has cgo enabled by default.
- `make` runs each recipe line in its own subshell; `CGO_ENABLED=0 go build …` is a single command on one line, so the env var correctly scopes to that `go build` — keep it one line.
- Reproducibility is best-effort here: `-s -w` removes debug metadata, but fully reproducible builds (trimmed paths) would also want `-trimpath`. **Decision:** match the canon flag string exactly (`-ldflags "-s -w"`) for now; note `-trimpath` as an available future hardening, do not add it unprompted.
- Do not introduce a build-tooling dependency (Taskfile, mage, etc.) — a plain Makefile over `go` is the whole ask and fits the standard-library-first canon.

## Out of scope

- Container image / server build (`Dockerfile`, the `docker-compose.yml` wiring) — task 15.
- Local-run documentation in `README.md` — task 14.
- End-to-end verification of a real write through the built binary — task 16.
- Release/versioning, version-stamping via `-ldflags -X`, cross-compilation matrices.

## Done when

- `make build` on macOS, with no C toolchain installed, produces a single static binary at `bin/proxy` with the `web/` GUI assets embedded and no cgo linkage.
- `make test` runs `go test ./...`; `make fmt` reformats and vets; `make tidy` syncs the module files.
- `bin/` is confirmed gitignored (or the note flags adding `/bin/`), so no built binary is ever committed.
