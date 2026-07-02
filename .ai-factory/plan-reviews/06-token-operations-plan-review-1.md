## Plan Review Summary

**Plan:** `06-token-operations.md`
**Files Reviewed:** plan + spec note `06-token-operations.md`, `internal/store/store.go`, ARCHITECTURE.md, ROADMAP.md, rules/base.md
**Risk Level:** 🟢 Low

The plan is a faithful, well-scoped decomposition of the spec note. It extends the existing `*Store` with `Create`/`List`/`Lookup`/`SetActive`/`Delete` over the already-created `tokens` table, matches the frozen schema exactly, and respects the architectural and canon constraints (label-blind write plane, plaintext token storage, instant-revocation via SQL `active=1` filter, no hashing/rotation).

### Context Gates

- **Architecture (`.ai-factory/ARCHITECTURE.md`)** — ✅ PASS. Store depends on nothing outside itself (plan uses only stdlib: `crypto/rand`, `encoding/base64`, `time`, `errors`, `database/sql`). No import of `proxy`/`admin`/`grafana`. Token shape (`name`, plaintext `token`, `active`, creation metadata) matches the documented "Token Store Shape". Deactivate = reversible soft-stop, delete = permanent — both preserved.
- **Rules (`.ai-factory/rules/base.md`)** — ✅ PASS. File `token.go` is snake_case; sentinel `ErrNotFound` + `fmt.Errorf("...: %w", …)` wrapping follows the stated error convention; store methods do not log token values (aligns with "Do not log token values"). Pure-Go, cgo-free preserved.
- **Roadmap (`.ai-factory/ROADMAP.md`)** — ✅ PASS. Directly implements the Phase 3 "Token operations" milestone; all five methods and the `Lookup` `active=1` guard and `UNIQUE(token)` constraint called out in the roadmap line are present. This is a `feat` with clear milestone linkage.

### Critical Issues

None. No missing migrations (schema already exists from task 05 and is create-if-not-exists; this task adds no columns), no wrong file paths, no incorrect API usage, no architectural boundary violations.

### Minor Observations (non-blocking)

- **`Lookup` "other error" path returns raw `err`.** Task 3 says any non-`ErrNoRows` scan/query error returns `Token{}, false, err` unwrapped, whereas the rest of the package wraps with `store: …: %w`. Harmless (the value is still an error) but slightly inconsistent with the stated convention. Consider wrapping as `store: lookup token: %w` for consistency — while keeping the `sql.ErrNoRows → (false, nil)` translation exactly as specified.
- **`ORDER BY created_at` has no tiebreaker.** `created_at` is Unix *seconds*, so multiple tokens minted within the same second have undefined relative order. Adding `, id` (or using a monotonic secondary key) would make list ordering stable. Purely cosmetic for the GUI; not required.
- **`SetActive` idempotent-reactivate is safe (confirmed, not a bug).** Worth recording: SQLite's `RowsAffected()` for `UPDATE` counts rows *matched* by the `WHERE` clause, not only rows whose value changed. So `SetActive(id, true)` on an already-active row still reports 1 row affected and will **not** falsely raise `ErrNotFound`. The plan's `RowsAffected()==0 → ErrNotFound` logic is therefore correct on this driver (`modernc.org/sqlite`).

### Positive Notes

- Correctly forces `crypto/rand` with explicit error propagation and an explicit ban on `math/rand` fallback — the right call for a security-sensitive token generator.
- Keeps the `active=1` filter in SQL (never load-then-check, never cache), which is exactly what makes deactivation effective on the next write per canon.
- `ErrNotFound` sentinel defined once and shared between `SetActive` and `Delete`, giving the future admin handler a clean `404` mapping without a silent-success trap.
- Correctly scopes out HTTP handlers, Bearer middleware, and Grafana validation to their own downstream milestones — no scope bleed.
- `Create` always inserts `active=1` and returns the fully populated struct (including the plaintext token for one-time copy), matching the management-endpoint contract in ROADMAP Phase 5.

PLAN_REVIEW_PASS
