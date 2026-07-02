# Code Review: Token management page (review 1)

**Files reviewed:** `web/app.js` (new), `web/index.html` (modified), against the live admin API (`internal/admin/tokens.go`, `internal/admin/auth.go`, `internal/store/token.go`, `internal/store/store.go`).

**Risk level:** 🟢 Low — no critical or runtime-breaking defects. The API contract is honored exactly (paths, field names, `201`/`204` codes, empty-body PATCH/DELETE, plain-text errors), the empty-body parsing gap flagged in the plan review is correctly handled, `created_at` units match (`time.Now().Unix()` → seconds, and `formatCreatedAt` multiplies by 1000), and rows are built with `createElement`/`textContent` so operator-supplied names cannot inject markup. Findings below are spec-fidelity and copy issues, not crashes.

## Findings

### 1. Stale token table is not cleared on `401` (Low–Medium)
`web/app.js` — `api()` (lines 85–89) and the caller catch blocks (`loadTokens` 190–193, `onDelete` 245–247, `onToggleActive` 232–234).

On a `401`, `api()` clears the held token, re-prompts, and rejects — but nothing clears the already-rendered `<tbody>`. Scenario: the operator connects successfully and the table renders (with every token's plaintext value visible); later the Grafana token expires; the operator toggles a checkbox or the next refresh runs → `PATCH`/`GET` returns `401` → the token is cleared and the "re-enter it" message shows, **but the full table of plaintext write-token values stays on screen**. This contradicts the plan's explicit requirement ("do not render stale data as if authorized" / "the UI never keeps acting as if it were still authorized"). The plaintext tokens were already visible to the same operator, so this is not a fresh disclosure — hence Low–Medium, not High — but it is a real deviation from the stated 401 contract.

*Suggested fix:* on the `401` branch in `api()` (or in `clearGrafanaToken`), also empty the table body — e.g. clear `#tokens tbody` and hide the new-token panel — so the unauthorized state shows no residual rows.

### 2. New-token panel copy is factually wrong for this system (Low)
`web/index.html:96` — "New token created — copy it now, **it won't be shown again in full**."

This message describes secret-once semantics, but this proxy deliberately stores tokens in plaintext and `GET /api/tokens` returns the full `token` value, which the table renders in full on every load (`app.js:135`). The token *is* shown again in full, permanently. The warning is misleading and, worse, could push an operator toward risky behavior (e.g. treating the panel as the only record). Reword to reflect reality — e.g. "New token created — copy it for your source's config." (the plaintext value also remains visible in the table below).

### 3. Minor UX nits (Nit — non-blocking)
- `web/app.js:250–256`: clicking **Connect** with an emptied token field silently reuses the previously held token (the `if (value)` guard skips `setGrafanaToken`). If the operator clears the field intending to "disconnect," nothing changes. Not a correctness bug, but the guard means the field can never be used to clear the token.
- `web/index.html`: the Grafana-token and name inputs are not inside a `<form>`, so pressing Enter does not trigger Connect/Create — the operator must click. Acceptable for a minimal GUI; noted for completeness.

## Positive notes
- Empty-body handling is correct: the `204` short-circuit (102–104) and the `res.text()` non-empty guard (105–109) prevent the `res.json()`-throws-on-empty bug that would otherwise make successful toggles bounce back.
- `401` vs. other-error branching is exactly as specified: `401` clears the token; `503`/other 5xx/network errors keep it and only show a message (85–100), so a momentarily-unreachable Grafana never wipes a valid token.
- Optimistic toggle with revert-on-failure (`onToggleActive` 225–235) correctly captures the prior state before the request and restores it in the catch.
- `navigator.clipboard?.writeText` is guarded (140) with the selectable/`user-select: all` cell as the documented fallback for plain-`http://` non-localhost origins.
- No token values are logged, placed in URLs, or persisted beyond in-memory + `sessionStorage`; never `localStorage`/cookie.
- `created_at` seconds→ms conversion verified against `store/token.go:57`.
