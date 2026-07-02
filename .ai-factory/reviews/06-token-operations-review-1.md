# Code Review: Token operations (round 1)

**Scope:** `internal/store/token.go` (new file; the only code change).
**Method:** `git diff HEAD` + `git status`; read `token.go` and `internal/store/store.go` in full; `go build ./...` and `go vet ./internal/store/` (both pass).

## Verdict

The implementation is a faithful, correct realization of the plan and spec note. All five methods (`Create`, `List`, `Lookup`, `SetActive`, `Delete`) plus the `Token` struct and the `newID`/`newToken` generators are present and behave as specified. No correctness, security, or runtime defects found.

## What was verified

- **`crypto/rand` usage** — both generators use `crypto/rand.Read`, propagate the error, and never fall back to `math/rand`. Encoding is `base64.RawURLEncoding` (unpadded), 16 bytes for `id`, 32 bytes for the `otlp_`-prefixed token. Matches spec exactly.
- **SQL injection** — every query is parameterized (`?` placeholders); no string concatenation of user input. Safe.
- **`Lookup` `active=1` filter lives in SQL** — the `WHERE token=? AND active=1` filter is in the query, not a load-then-check in Go, and there is no caching, so deactivation is effective on the very next request per canon.
- **`sql.ErrNoRows` handling** — `Lookup` translates a miss to `(Token{}, false, nil)` via `errors.Is`, keeping the normal-401 path distinct from a real error. Any other error is wrapped as `store: lookup token: %w` (this is actually stricter/better than the plan, which permitted a raw `err`).
- **`List` returns all rows** — no `active` filter; iterates, checks `rows.Err()` after the loop, `defer rows.Close()`. `active INTEGER` mapped to `bool` via `active != 0`. Correct.
- **`SetActive` / `Delete` not-found signalling** — both check `RowsAffected()` and return the shared `ErrNotFound` sentinel on zero rows, giving the future admin handler a clean 404 mapping. `ErrNotFound` is defined once and reused.
- **`RowsAffected()` on a no-op reactivate is safe** — verified: SQLite's `sqlite3_changes()` counts rows matched/processed by an UPDATE, not only rows whose stored value differs. So `SetActive(id, true)` on an already-active row reports 1 affected and will not falsely raise `ErrNotFound`. The logic is correct on the `modernc.org/sqlite` driver.
- **Error wrapping** — consistent `store: …: %w` style matching `store.go`.
- **Architecture** — package imports only stdlib (`crypto/rand`, `database/sql`, `encoding/base64`, `errors`, `fmt`, `time`); no dependency on `proxy`/`admin`/`grafana`. Stays cgo-free.
- **Scope** — no HTTP handlers, Bearer middleware, Grafana validation, or hashing/rotation leaked in; correctly deferred to downstream milestones.

## Non-blocking observations (no action required)

- **`ORDER BY created_at` has no tiebreaker.** `created_at` is Unix *seconds*, so tokens minted in the same second have undefined relative order in `List()`. Purely cosmetic for the GUI; adding `, id` would make it stable. Not a bug.
- **`Create` does not retry on an `id`/`token` collision.** A collision surfaces as a wrapped `UNIQUE`/PK constraint error rather than an automatic retry. With 128-bit (`id`) and 256-bit (`token`) random values this is astronomically unlikely, and surfacing the error is the spec-mandated behavior. Not a bug.

REVIEW_PASS
