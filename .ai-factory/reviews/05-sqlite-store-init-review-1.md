# Code Review: 05 — SQLite store init

**Plan:** `.ai-factory/plans/05-sqlite-store-init.md`
**Changed/new code reviewed:** `go.mod`, `go.sum`, `internal/store/store.go`
**Verified against:** `internal/config/config.go` (conventions), installed toolchain `go1.19.3`, live build + runtime smoke test.

## Summary

**Risk Level:** 🟢 Low — no findings.

The implementation matches the plan and the note exactly, and I confirmed it works at runtime rather than only inspecting the diff:

- `CGO_ENABLED=0 go build ./...` → exit 0 (cgo-free static build preserved; pinned `modernc.org/sqlite v1.28.0` on `go 1.19`, `go.mod` directive unchanged).
- Ran a throwaway smoke test against `internal/store`:
  - Nested path (`<tmp>/sub/proxy.db`) — `os.MkdirAll` creates the parent dir, `Open` succeeds.
  - `PRAGMA journal_mode` → `wal`, `PRAGMA busy_timeout` → `5000` — confirms the DSN-scoped pragmas actually take effect on pooled connections (not just on one arbitrary connection).
  - `tokens` table present in `sqlite_master` after `Open`.
  - A second `Open` on the same path succeeds — idempotent `CREATE TABLE IF NOT EXISTS`.
  - `Close()` returns nil.

## Correctness review

- **Handle-leak discipline:** `db.Close()` is called on both the `Ping` failure and the schema-`Exec` failure paths before returning the error, so a failed `Open` leaks no `*sql.DB`. Correct.
- **Lazy-open trap mitigated:** `sql.Open` is lazy; `db.Ping()` forces a real connection so a bad/locked path fails at startup, not at first query. Present and correct.
- **Pragmas connection-scoped via DSN:** carrying `_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)` in the DSN (rather than a one-shot `PRAGMA` statement) is the right call for `database/sql` pooling — verified both pragmas apply on a freshly-pooled connection.
- **Parent-dir skip on `.`:** `filepath.Dir("./proxy.db")` is `"."`, correctly skipping `MkdirAll` for the default path; a nested configured path is created. Verified.
- **Schema:** matches the spec (`id TEXT PK`, `name NOT NULL`, `token NOT NULL UNIQUE`, `active INTEGER NOT NULL DEFAULT 1`, `created_at INTEGER NOT NULL`); `IF NOT EXISTS` only, no destructive DDL — safe on every startup.
- **Error wrapping / conventions:** package doc, exported-identifier docs, and `fmt.Errorf("store: ...: %w", err)` wrapping faithfully mirror `internal/config`. Package/file naming (`store` / `store.go`) conform.
- **Scope:** no CRUD/lookup, no `cmd/proxy` wiring, no migration framework — all correctly deferred to note 06. `internal/store` compiles standalone though currently unimported, which is expected this milestone.

## Informational (no action required)

- **DSN URL-safety:** `path + "?_pragma=..."` is safe for the default `./proxy.db`; a configured path containing `?`, `#`, or spaces would corrupt the query string. Already flagged as an out-of-scope operator edge case in both plan reviews; not introduced by this change and not blocking.
- **WAL side files:** WAL mode produces `proxy.db-wal` / `proxy.db-shm` alongside the DB — expected behavior, worth keeping in mind for the future `DB_PATH` volume-mount deployment task, not a defect here.

REVIEW_PASS
