# Code Review: Token management page (review 2)

**Files reviewed:** `web/app.js` (new, 288 lines), `web/index.html` (modified), re-checked against the live admin API (`internal/admin/tokens.go`, `internal/admin/auth.go`, `internal/store/token.go`, `internal/store/store.go`).

**Risk level:** 🟢 Pass — all three review-1 findings are resolved and the refactor introduces no new defects.

## Verification of prior findings

1. **Stale token table on `401` (was Low–Medium) — FIXED.** `clearGrafanaToken()` now calls the new `clearTokenTable()` (`app.js:30`, `37–46`), which empties `#tokens tbody` and hides the new-token panel. Every `401` path routes through `api()` → `clearGrafanaToken()`, so an expired token no longer leaves plaintext write-tokens on screen.
2. **Misleading new-token copy (was Low) — FIXED.** `index.html:96` now reads "New token created — copy it for your source's config:", dropping the false "won't be shown again in full" claim (the value is stored plaintext and remains visible in the table).
3. **UX nits (were Nit) — FIXED.** Both controls are now `<form>` elements (`token-bar`, `create-form`) with `submit` handlers that `preventDefault()` (`app.js:266–281`), so Enter submits. Empty Connect now calls `setGrafanaToken("")` and `clearTokenTable()`, giving a clean "disconnect" instead of silently reusing the old token.

## Re-review of the refactor (no new findings)

- **Form submit wiring:** `connect` and `create` are `type="submit"` inside their forms; both handlers call `preventDefault()`, so no full-page navigation occurs. Correct.
- **Empty-connect path (`app.js:270–275`):** `setGrafanaToken("")` clears memory + `sessionStorage`; `clearTokenTable()` clears the grid; subsequent `loadTokens()` short-circuits on the empty token. Coherent disconnect behavior.
- **Empty-body handling unchanged and still correct:** `204` short-circuits to `null` (118–120); PATCH `200` empty body falls to `res.text()` → `""` → `null` (121–124); non-OK handled before any body parse (107–116). The success toggle does not spuriously revert.
- **Toggle revert reference stays valid:** on failure no re-render occurs, so `checkbox` is still the live node when `checked = previous` runs (241–251); on success `loadTokens()` re-renders with fresh nodes. No dangling-reference bug.
- **Security posture intact:** Bearer-header auth (not cookies) makes the admin API inherently CSRF-safe; token never logged, never in a URL, memory + `sessionStorage` only; rows built via `textContent`, closing the name-injection XSS vector.
- **Contract match confirmed:** paths, field names, `201`/`204`/empty-`200` codes, plain-text errors, and `created_at` epoch-seconds (`store/token.go:57` → `formatCreatedAt` ×1000) all line up. Empty list encodes as `[]` (Go `make([]…, 0)`), so `renderTokens` never receives a bare `null`.

No bugs, security issues, or correctness problems remain.

REVIEW_PASS
