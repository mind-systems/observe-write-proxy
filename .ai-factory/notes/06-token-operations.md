# 06 — Token operations

**Task:** ROADMAP → Phase 3 — Token store → "Token operations"
**Depth:** medium

## Goal

Give `internal/store` the four operations the proxy needs over the `tokens` table: create a write token, list all tokens, toggle a token's active flag, and look up a presented token for per-request write auth. Together they realise the canon's "never delete, only deactivate" model and the rule that a write token is valid only when it exists AND `active=1`.

## Design

- `Token` struct (the shape returned by create/list/lookup):
  - `ID string`
  - `Name string`
  - `Token string` (plaintext value — readable later in the GUI, per canon)
  - `Active bool`
  - `CreatedAt int64` (Unix seconds)
  Map `active INTEGER` ↔ `bool` and `created_at INTEGER` ↔ `int64` at the scan boundary.

- `Create(name string) (Token, error)` on `*Store`:
  - Generate `id`: random, URL-safe (e.g. `base64url` of ~16 random bytes from `crypto/rand`).
  - Generate `token = "otlp_" + base64url(32 random bytes)` using `crypto/rand`. Use RawURLEncoding (no `=` padding) so the token is a clean opaque string.
  - `created_at = time.Now().Unix()`.
  - `INSERT INTO tokens(id, name, token, active, created_at) VALUES(?, ?, ?, 1, ?)` — always inserted active.
  - Return the fully populated `Token`.

- `List() ([]Token, error)`:
  - `SELECT id, name, token, active, created_at FROM tokens` (consider `ORDER BY created_at`).
  - Return every row including `name`, plaintext `token`, `active`, `created_at`. **Decision:** return all tokens (active and inactive) — the admin GUI needs to show and reactivate inactive ones, so List must not filter on `active`.

- `SetActive(id string, active bool) error`:
  - `UPDATE tokens SET active=? WHERE id=?` (bind `1`/`0`).
  - This is the single deactivate/reactivate operation. There is **no** delete method anywhere in the package.
  - **Decision:** return an error (or a not-found signal) when no row matched, so the admin handler can answer `404` for an unknown id rather than silently succeeding — check `RowsAffected()`.

- `Lookup(token string) (Token, bool, error)`:
  - `SELECT id, name, token, active, created_at FROM tokens WHERE token=? AND active=1`.
  - Called on every write request. Returns the row with `ok=true` only when the token exists AND is active; an inactive or unknown token yields `ok=false` (with `nil` error — absence is not a failure).
  - The `AND active=1` filter must live in the query so deactivation takes effect on the very next request with no caching.

## Edge cases / watch

- **Lookup must filter on `active`** in SQL — never load the row then check the flag in Go, and never cache results; the canon requires deactivation to be effective on the next request.
- **`crypto/rand` errors:** `rand.Read` can fail; propagate that error from `Create` rather than ignoring it or falling back to `math/rand`. A weak token here is a security hole.
- **Unique constraint on `token`:** the `UNIQUE` index can in principle reject an insert on a (astronomically unlikely) 32-byte collision; surface the constraint error from `Create` rather than masking it.
- **No-row updates:** distinguish "updated" from "id not found" in `SetActive` via `RowsAffected()` so the admin plane can map it to `404`.
- **`ok=false` vs error:** `Lookup` of an absent/inactive token is the normal `401` path, not an error — keep `sql.ErrNoRows` from leaking out as a real error; translate it to `ok=false, err=nil`.

## Out of scope

- HTTP handlers for `/api/tokens` and the per-request `401` enforcement on `/v1/logs` — those live in the admin/proxy planes; this note is the store layer only.
- Grafana-token validation for the admin plane.
- Any token hashing/rotation — tokens are stored plaintext by canon decision.

## Done when

- A token can be created (`otlp_`-prefixed, `crypto/rand`-derived, inserted active) and the populated `Token` is returned.
- `List()` returns all tokens — active and inactive — with name, plaintext token, active flag, and created_at.
- `SetActive(id, false)` then `SetActive(id, true)` toggles the row without deleting it; an unknown id is reported, not silently ignored.
- `Lookup` returns `ok=true` for an active token and `ok=false` for an inactive or unknown one, filtering on `active` in the query.
- No operation in the package issues a `DELETE`.
