## Code Review Summary

**Files Reviewed:** Plan `12-token-management-page.md` (revision 2) against the live codebase — `internal/admin/tokens.go`, `internal/admin/static.go`, `internal/admin/auth.go`, `internal/store/token.go`, `cmd/proxy/main.go`, `web_embed.go`, `web/index.html`, plus `ARCHITECTURE.md` / `ROADMAP.md` / `rules/` and the round-1 review.
**Risk Level:** 🟢 Low

This revision resolves every issue raised in review-1. All five prior findings are now written directly into the task text, and each one matches the actual handler behavior I re-verified in the code. No new gaps found; the API contract, routing, status codes, and security posture are all accurate.

### Context Gates

- **Architecture** (`ARCHITECTURE.md`): PASS. The page stays at the public/ungated `/` route (`static.go` + `newMux` line 114), drives all mutations through the Grafana-gated `/api/tokens[/]` API, and adds no server code. No plane mixing — the Grafana admin token never touches the write path.
- **Rules** (`rules/`): PASS. Vanilla no-build `web/app.js` respects the single-static-binary / no-bundler / no-cgo constraints. No naming or structural conflicts.
- **Roadmap** (`ROADMAP.md`): PASS. Implements the Phase 6 "Token management page" item and honors its guard list (401 handling, revert-on-failed-toggle, never persist the Grafana token). The referenced spec note `.ai-factory/notes/12-token-management-page.md` **exists** — the WARN from review-1 is cleared.

### Verification of review-1 findings (all resolved)

1. **Empty-body parsing** — Task 2 now mandates deciding the branch on `res.status`/`res.ok` *before* touching the body, parsing JSON only for OK responses that actually carry one, and explicitly calls out that a mis-parse would wrongly trigger Task 4's revert. Confirmed against code: `PATCH` returns `200` empty (`tokens.go:134`), `DELETE` returns `204` (`tokens.go:145`); only `GET` (array, line 82) and `POST` (`201`, line 98) carry JSON. ✅
2. **Non-401 statuses (503)** — Task 2 now distinguishes `401` (clear + re-prompt) from `503`/other 5xx/network (show message, keep held token). Matches `auth.go`: `503` "grafana unreachable" on validator error (line 36), `401` only for missing/invalid token (lines 30, 40). ✅
3. **Plain-text error bodies** — Covered by the "branch before touching the body" rule; all error paths use `http.Error` (text/plain). ✅
4. **`navigator.clipboard` in non-secure contexts** — Task 3 now requires guarding `navigator.clipboard?` with the selectable cell as fallback. ✅
5. **Rehydrated-but-invalid session token** — Task 3 now routes a stale `sessionStorage` token through the same `401` clear+re-prompt path instead of rendering an empty grid. ✅

### Additional verification

- **Routing:** `POST`/`GET` → `/api/tokens` (exact, `Collection`), `PATCH`/`DELETE` → `/api/tokens/<id>` (prefix `/api/tokens/`, `Item`). Matches `newMux` lines 112–113 and the `Item` id-parse at `tokens.go:110`. ✅
- **Field names/types:** `id`, `name`, `token`, `active` (bool), `created_at` (int64) — match `tokenResponse` (`tokens.go:39-45`). ✅
- **Embed:** `//go:embed web` picks up a new `web/app.js` with no Go change; `http.FileServer` serves it at `/app.js`. No migration, no route addition needed. ✅

### Implementation nuance (non-blocking, not a plan defect)

- `created_at` is **epoch seconds** (`store.Create` uses `time.Now().Unix()`, `token.go:57`), while JS `Date` expects **milliseconds**. Task 3's "formatted from the epoch value into a readable local timestamp" is accurate as written; the implementer just needs `new Date(created_at * 1000)`, not `new Date(created_at)`, or the timestamp renders as ~1970. Worth keeping in mind during implementation.

### Positive Notes

- Every review-1 finding was folded into the task text with the *reasoning* preserved (e.g. why an empty-body mis-parse specifically breaks the toggle), not just patched over — the plan now teaches the implementer the failure mode.
- Security posture remains sound: Grafana token in memory + `sessionStorage` only (never `localStorage`/cookie/markup), never logged, never in URLs; rows built via `createElement`/`textContent`, closing the XSS vector on operator-supplied names.
- Live-toggle semantics (optimistic flip, revert-on-failure, no save step; delete kept distinct behind `confirm()`) faithfully mirror the store's reversible-deactivate vs. permanent-delete model.
- Task dependency ordering (shell → auth/fetch helper → render → mutations) is clean and correctly serialized.

PLAN_REVIEW_PASS
