# Plan: SQLite store init

## Context
Stand up the write-plane persistence layer: a new `internal/store` package that opens (or creates) a SQLite database via the pure-Go `modernc.org/sqlite` driver, guarantees the `tokens` table exists, and returns a `*Store` handle that later token-operation tasks build on. Keeps the binary cgo-free.

## Settings
- Testing: no
- Logging: minimal
- Docs: no

## Tasks

### Phase 1: Store package

- [x] **Task 1: Add the `modernc.org/sqlite` dependency**
  Files: `go.mod`, `go.sum`
  Add `modernc.org/sqlite` as the module's only new dependency. It must be a pure-Go driver (no cgo) to preserve the `CGO_ENABLED=0` invariant. Do not add `mattn/go-sqlite3` or any cgo driver.
  - **Pin to `v1.28.0`** — this module declares `go 1.19`, and the installed toolchain is `go1.19.3`. Any `modernc.org/sqlite` ≥ v1.29.0 requires Go ≥ 1.20 (latest v1.53.0 requires Go 1.25), and Go 1.19 predates `GOTOOLCHAIN` auto-download, so an unpinned `go get` would select the latest and break the build. `v1.28.0` is the last release compatible with Go 1.19 and pulls only pure-Go transitive deps (`modernc.org/libc`, `memory`, `opt`, `strutil`, `token`).
  - Run `go get modernc.org/sqlite@v1.28.0` then `go mod tidy` so `go.mod`/`go.sum` are consistent and the package resolves. The `go 1.19` directive in `go.mod` must remain unchanged; if `go mod tidy` tries to raise it, the wrong version was selected.
  - Raising the sqlite version later would require a deliberate, separate decision to bump the `go` directive and install a matching toolchain — out of scope here.

- [x] **Task 2: Implement `internal/store` with `Open`, `Store`, and `Close`** (depends on Task 1)
  Files: `internal/store/store.go`
  Create the `store` package following the existing `internal/config` conventions (package doc comment describing purpose, doc comments on exported identifiers, wrapped `fmt.Errorf("store: ...: %w", err)` errors).
  - Blank-import the driver for side-effect registration: `import _ "modernc.org/sqlite"` (registers under driver name `"sqlite"`).
  - Define `type Store struct { db *sql.DB }` — sole owner of the handle; later token operations attach as methods on `*Store`.
  - `Open(path string) (*Store, error)`:
    - Ensure the parent directory exists before opening: `os.MkdirAll(filepath.Dir(path), 0o755)`, skipping when `filepath.Dir(path)` is `"."` so the default `./proxy.db` needs no mkdir.
    - Build a DSN that carries the pragmas so every pooled connection inherits them (pragmas are connection-scoped under `database/sql` pooling): append `?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)` to the path. Open with `sql.Open("sqlite", dsn)`.
    - Force a real connection to surface a bad/locked path at startup rather than at first query (`sql.Open` is lazy): call `db.Ping()`; on error close the handle and return a wrapped error.
    - Run the schema (Task 3) idempotently, then return `&Store{db: db}`. On any failure after `sql.Open`, close `db` before returning so a failed `Open` leaks no handle.
  - `Close() error`: close the underlying `*sql.DB`, returning its error wrapped.

- [x] **Task 3: Create the `tokens` schema idempotently** (depends on Task 2)
  Files: `internal/store/store.go`
  Inside `Open` (after the successful ping), execute a single `CREATE TABLE IF NOT EXISTS tokens (...)` statement via `db.Exec`, defined as a package-level const string:
  - `id TEXT PRIMARY KEY`
  - `name TEXT NOT NULL`
  - `token TEXT NOT NULL UNIQUE`
  - `active INTEGER NOT NULL DEFAULT 1`
  - `created_at INTEGER NOT NULL`
  `active` is the 1/0 flag backing reversible deactivation; the `UNIQUE` on `token` rejects duplicate token strings at the storage layer. Use `IF NOT EXISTS` only — never `DROP` or destructive migration — so the statement is safe to run on every startup.
  Manual verification (no automated tests this milestone): call `Open` twice against a fresh path — the first creates `proxy.db` and the `tokens` table, the second succeeds without error (idempotent schema) — then `Close`; confirm the build stays cgo-free (`CGO_ENABLED=0 go build ./...` exits 0).

## Notes
Out of scope (later tasks): token CRUD/lookup operations (note 06), wiring the store into `cmd/proxy` and threading `config.DBPath`, and any migration/versioning framework. Single commit at the end: "Add SQLite token store initialization".
