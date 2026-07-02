# Plan: Build automation

## Context
Add a repo-root `Makefile` that captures the project's build/run/test/fmt/tidy commands so the single static, cgo-free `bin/proxy` binary (with `web/` GUI embedded) is produced identically every time. Spec: `.ai-factory/notes/13-build-automation.md`.

## Settings
- Testing: no
- Logging: minimal
- Docs: no

## Tasks

### Phase 1: Makefile

- [x] **Task 1: Add root Makefile with build/run/test/fmt/tidy targets**
  Files: `Makefile`
  Create a new `Makefile` at the repo root. Declare all targets in `.PHONY` (none map to real tracked files). Recipes are thin wrappers over `go`:
  - `build` — `CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/proxy ./cmd/proxy`. Keep `CGO_ENABLED=0` inline on this one recipe line (not exported globally) so the pure-Go / no-cgo invariant travels with the Makefile; it is the contract that ties to the `modernc.org/sqlite` driver. `-ldflags "-s -w"` strips the symbol table and DWARF info. `web/` assets ride along automatically via the existing `//go:embed` (see `web_embed.go`) — no separate bundling step. Match the canon flag string exactly; do NOT add `-trimpath` unprompted.
  - `run` — `go run ./cmd/proxy`. Deliberately uses `go run` (not a dependency on `build`) so quick local iteration picks up source changes directly and does not litter `bin/`. Env vars (`PROXY_LISTEN`, `LOKI_URL`, `GRAFANA_URL`, `DB_PATH`) are inherited from the caller's shell; bake no flags in.
  - `test` — `go test ./...`.
  - `fmt` — run `gofmt -l -w .` then `go vet ./...` (vet folded into `fmt`; no separate `vet` target, keeping the command surface minimal).
  - `tidy` — `go mod tidy`.
  Do not introduce any build-tooling dependency (Taskfile, mage, etc.) — a plain Makefile over `go` is the whole ask.

- [x] **Task 2: Confirm `bin/` is gitignored**
  Files: `.gitignore`
  Verify `/bin/` is present in `.gitignore` so the built `bin/proxy` binary is never committed. It is already present at `.gitignore:8` (`/bin/`) — no edit is required; confirm and leave as-is. (The spec note predates this line being added.)
