# Plan: Token operations

## Context
Give `internal/store` the full write-token API on `*Store` — mint, list, toggle active, delete, and per-request lookup — over the existing `tokens` table, so the write and admin planes can authenticate and manage tokens.

## Settings
- Testing: no
- Logging: minimal
- Docs: no

## Tasks

### Phase 1: Token model and generation

- [x] **Task 1: Add the `Token` struct and random-ID/token generators**
  Files: `internal/store/token.go`
  In a new `token.go` in package `store`, define the row shape returned by create/list/lookup:
  ```go
  type Token struct {
      ID        string
      Name      string
      Token     string
      Active    bool
      CreatedAt int64
  }
  ```
  Add two unexported helpers built on `crypto/rand` + `encoding/base64` (`base64.RawURLEncoding`, no padding — matches the clean opaque-string requirement):
  - `newID() (string, error)` → RawURLEncoding of exactly **16** `crypto/rand` bytes.
  - `newToken() (string, error)` → `"otlp_" + RawURLEncoding(32 crypto/rand bytes)`.
  Both must propagate any `rand.Read` error rather than ignoring it or falling back to `math/rand` (a weak token is a security hole). Follow the existing `fmt.Errorf("store: …: %w", err)` error-wrapping style used in `store.go`.

### Phase 2: Store methods

- [x] **Task 2: Implement `Create(name)`** (depends on Task 1)
  Files: `internal/store/token.go`
  Method on `*Store`: `Create(name string) (Token, error)`.
  - Generate `id` via `newID()` and `token` via `newToken()`, propagating their errors.
  - `createdAt := time.Now().Unix()`.
  - `INSERT INTO tokens(id, name, token, active, created_at) VALUES(?, ?, ?, 1, ?)` — always inserted active (`active=1`).
  - Surface any error (including the `UNIQUE` constraint on `token`, however unlikely) wrapped as `store: create token: %w`.
  - Return the fully populated `Token{ID, Name: name, Token: token, Active: true, CreatedAt: createdAt}`.

- [x] **Task 3: Implement `List()` and `Lookup(token)`** (depends on Task 1)
  Files: `internal/store/token.go`
  - `List() ([]Token, error)`: `SELECT id, name, token, active, created_at FROM tokens ORDER BY created_at`. Return **every** row (active and inactive — the GUI must show and reactivate inactive ones); do not filter on `active`. Map `active INTEGER`↔`bool` and `created_at INTEGER`↔`int64` at the scan boundary. Check `rows.Err()` after iteration and `defer rows.Close()`.
  - `Lookup(token string) (Token, bool, error)`: `SELECT id, name, token, active, created_at FROM tokens WHERE token=? AND active=1` via `QueryRow`. The `AND active=1` filter **must live in SQL** (never load then check in Go, never cache) so deactivation takes effect on the very next request. On `sql.ErrNoRows` return `Token{}, false, nil` (absent/inactive is the normal `401` path, not an error). On a hit return the row with `ok=true`. Any other scan/query error → `Token{}, false, err`.

- [x] **Task 4: Implement `SetActive(id, bool)` and `Delete(id)`** (depends on Task 1)
  Files: `internal/store/token.go`
  - `SetActive(id string, active bool) error`: `UPDATE tokens SET active=? WHERE id=?`, binding `1`/`0`. This is the reversible soft-stop (deactivate/reactivate) — the row is kept. Check `RowsAffected()`; when zero rows matched, return a distinguishable not-found error (e.g. an exported `ErrNotFound = errors.New("store: token not found")`) so the admin handler can map it to `404` instead of a silent success.
  - `Delete(id string) error`: `DELETE FROM tokens WHERE id=?` — the permanent hard-remove (only method that issues a `DELETE`). Same `RowsAffected()` check returning `ErrNotFound` on no match.
  Define the shared `ErrNotFound` sentinel once (in `token.go`) and reuse it in both methods.

## Notes
- Out of scope (do not implement here): HTTP handlers for `/api/tokens`, the `/v1/logs` Bearer middleware, Grafana validation, and any token hashing/rotation — tokens are stored plaintext by canon.
- Single logical unit (< 5 tasks, one file) → one commit at the end; no commit plan needed.
