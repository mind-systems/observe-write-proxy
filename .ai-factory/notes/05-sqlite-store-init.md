# 05 — SQLite store init

**Task:** ROADMAP → Phase 3 — Token store → "SQLite store init"
**Depth:** low

## Goal

Stand up the persistence layer for write-plane tokens: a new `internal/store` package that opens (or creates) a SQLite database at a configured path, guarantees the `tokens` table exists, and returns a usable handle that later tasks build CRUD-style operations on. This is the foundation the per-request write auth check reads from.

## Design

- New package `internal/store`.
- Driver: pure-Go `modernc.org/sqlite`, imported for its side-effect driver registration (`import _ "modernc.org/sqlite"`) so it registers under the driver name `"sqlite"`. The DB is opened with `sql.Open("sqlite", path)`. **Decision:** use `modernc.org/sqlite` rather than the cgo `mattn/go-sqlite3` because the binary must stay `CGO_ENABLED=0`; this is the only new dependency the store introduces and it keeps the build cgo-free and statically linkable.
- `Open(path string) (*Store, error)`:
  - Ensure the parent directory of `path` exists (see Edge cases) before opening.
  - `db, err := sql.Open("sqlite", path)`. Note `sql.Open` is lazy, so issue a `db.Ping()` (or run the schema statement, which forces a real connection) to surface a bad path / locked file early.
  - Apply pragmas on the connection: `PRAGMA journal_mode=WAL;` and a `PRAGMA busy_timeout=<ms>;` (a few seconds, e.g. `5000`). WAL improves read/write concurrency for the per-request lookups; `busy_timeout` makes concurrent writers wait briefly instead of failing immediately with `SQLITE_BUSY`.
  - Run the create-if-not-exists schema (below), idempotently.
  - Return `&Store{db: db}`.
- `Store` wraps a `*sql.DB` (unexported field). It is the single owner of the DB handle; later operations are methods on `*Store`.
- `Close() error` closes the underlying `*sql.DB`.
- Schema (executed as `CREATE TABLE IF NOT EXISTS`):
  - table `tokens`:
    - `id TEXT PRIMARY KEY`
    - `name TEXT NOT NULL`
    - `token TEXT NOT NULL UNIQUE`
    - `active INTEGER NOT NULL DEFAULT 1`
    - `created_at INTEGER NOT NULL`
  - `active` is the boolean (1 = active, 0 = inactive) that backs deactivation; rows are never deleted, only flipped. `token` carries a `UNIQUE` constraint so duplicate token strings are rejected at the storage layer.

## Edge cases / watch

- **Parent directory:** the default config path (`./proxy.db`) sits in CWD and needs no mkdir, but an operator may configure a nested path. `Open` should `os.MkdirAll(filepath.Dir(path), 0o755)` (skipping when dir is `.`) so a configured path doesn't fail with "unable to open database file". Document the requirement either way.
- **Lazy open hides errors:** `sql.Open` never touches the file; without a ping/exec a broken path surfaces only on first query. Force a real connection in `Open` so startup fails loudly rather than at first write.
- **Pragma per connection:** `database/sql` pools connections; pragmas like `busy_timeout` are connection-scoped. Set them via the DSN/connection string where possible (e.g. `path?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)`) so every pooled connection inherits them, rather than running them once on an arbitrary connection.
- **Idempotency:** schema creation must be safe to run on every startup — `IF NOT EXISTS` only; never `DROP`/migrate destructively here.

## Out of scope

- All token operations (create, list, set-active, lookup) — those are note 06.
- Wiring the store into `cmd/proxy` and threading the configured SQLite path from `internal/config`.
- Any schema migration framework / versioning beyond create-if-not-exists.

## Done when

- `store.Open(path)` creates the DB file (and parent dir) when absent, opens it when present, applies the WAL + busy_timeout pragmas, and ensures the `tokens` table — including the `active` column — exists.
- A second call to `Open` against the same path succeeds without error (idempotent schema).
- `Open` returns a usable `*Store` whose `Close()` releases the handle; `modernc.org/sqlite` is the only new dependency and the build stays cgo-free.
