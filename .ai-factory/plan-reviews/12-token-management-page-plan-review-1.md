## Code Review Summary

**Files Reviewed:** Plan `12-token-management-page.md` against the live codebase (`internal/admin/tokens.go`, `internal/admin/static.go`, `internal/admin/auth.go`, `cmd/proxy/main.go`, `internal/store/token.go`, `web_embed.go`, `web/index.html`, ROADMAP/ARCHITECTURE/RULES)
**Risk Level:** 🟡 Medium

The plan is well-scoped and its assumptions about the API surface are almost entirely correct: paths (`/api/tokens`, `/api/tokens/<id>`), JSON field names (`id`, `name`, `token`, `active`, `created_at`), the `POST → 201` / `DELETE → 204` codes, the plaintext-token-in-`token` convention, and the public/ungated `/` page all match the implemented handlers. Adding `web/app.js` needs no Go change — `//go:embed web` in `web_embed.go` picks up any new file under `web/` at build time, and `http.FileServer` serves it at `/app.js`. There is **one concrete correctness gap** (empty-body parsing) that will make the toggle and delete flows misbehave if implemented literally, plus a few smaller robustness notes.

### Context Gates

- **Architecture** (`ARCHITECTURE.md`): PASS. Plan keeps `/` public and drives everything client-side via the Grafana-gated `/api/tokens` API — consistent with the two-plane split and the "static GUI served by the admin plane" boundary. No new server code, no plane mixing.
- **Rules** (`rules/base.md`): PASS. Rules are Go/backend-oriented; the vanilla no-build `web/app.js` respects the "one static binary, no cgo, no bundler" spirit. No naming/structure conflicts.
- **Roadmap** (`ROADMAP.md`): PASS. Directly implements the Phase 6 "Token management page" item; matches its guard list (handle 401, revert checkbox on failed toggle, never persist the Grafana token). Note: the roadmap references a spec note `.ai-factory/notes/12-token-management-page.md` — WARN if that note does not exist, the implementer loses the detailed contract the plan defers to.

### Critical Issues

**1. `api()` will throw / mis-report on empty response bodies (PATCH 200 and DELETE 204).**
Task 2 says the `api()` helper "returns the parsed response," and Task 4 wires toggle/delete through it. But:
- `PATCH /api/tokens/<id>` returns **`200` with an empty body** (`tokens.go` calls `w.WriteHeader(http.StatusOK)` and writes nothing).
- `DELETE /api/tokens/<id>` returns **`204` with no body**.

If `api()` unconditionally calls `res.json()`, it throws `SyntaxError: Unexpected end of JSON input` on both. For the live toggle this is worse than a crash: the throw looks like a failure, so Task 4's "revert the checkbox on failure" path fires **even though the server accepted the change** — the operator sees the toggle bounce back on every successful flip. Delete would surface a spurious error after actually deleting.
Fix in the plan: `api()` must only parse a body when there is one — e.g. branch on `res.status === 204` / empty `Content-Length`, or read `res.text()` and parse only if non-empty. Only `GET` (array) and `POST` (`201` token object) carry JSON.

### Suggestions (non-blocking)

**2. Admin gate returns non-401 statuses the plan does not account for.**
`RequireGrafana` returns **`503` "grafana unreachable"** when Grafana can't be reached (transport/5xx), and `401` only for missing/invalid tokens. The plan centralizes "401 → clear token + re-prompt" but says nothing about `503`. Treating a `503` as a 401 would wrongly wipe a valid held token and prompt re-entry when the token is fine and Grafana is merely down. The status-line handling should distinguish "auth failed (401) → clear + re-prompt" from "other error (5xx/network) → show message, keep the held token."

**3. Error response bodies are plain text, not JSON.**
All admin error paths use `http.Error(...)`, which emits `text/plain` (e.g. `unauthorized`, `internal error`, `not found`). This reinforces issue #1: `api()` must not attempt `res.json()` on non-OK responses. Decide the success/failure branch on `res.ok`/status *before* parsing.

**4. `navigator.clipboard` is unavailable in non-secure contexts.**
Task 3 uses `navigator.clipboard.writeText` for the copy affordance. `navigator.clipboard` is `undefined` on plain `http://` origins that aren't `localhost` — which is exactly the "exposed on a shared server, reachable by mobile/browser clients" deployment described in the repo's CLAUDE.md unless it sits behind an HTTPS reverse proxy. The plan already mandates a selectable/monospace token cell, so manual copy is a natural fallback; just have the copy handler guard for `navigator.clipboard?` and degrade instead of throwing.

**5. Minor: same-tab reload rehydration vs. the initial no-token load.**
Task 2 rehydrates from `sessionStorage` and Task 3 loads the list "if a token is present." Worth making explicit that a rehydrated-but-now-invalid token flows through the same `401` path (clear + re-prompt) rather than rendering an empty/broken table — otherwise a stale session token silently yields an empty grid.

### Positive Notes

- API contract is accurately reverse-engineered: field names, paths, and status codes all match `tokens.go` and the ROADMAP entry exactly — no invented endpoints or wrong casing.
- Security posture is sound and matches the architecture: Grafana token in memory + `sessionStorage` only (never `localStorage`/cookie/markup), never logged, never in URLs; rows built via `createElement`/`textContent` rather than HTML string interpolation, closing the obvious XSS vector on a page that renders operator-supplied names.
- Correctly recognizes no server/DB change is needed — the embed picks up `web/app.js` automatically; no migration, no route addition.
- Live-toggle semantics (no save step, optimistic flip with revert-on-failure, delete kept distinct behind `confirm()`) faithfully mirror the store's reversible-deactivate vs. permanent-delete model in `ARCHITECTURE.md`.
- Task dependency ordering (shell → auth/fetch helper → render → mutations) is clean and correctly serialized.
