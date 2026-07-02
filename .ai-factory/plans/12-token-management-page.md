# Plan: Token management page

## Context
Build the vanilla, build-step-free admin GUI (`web/index.html` + `web/app.js`) that lets a single operator paste a Grafana token and manage the proxy's write tokens live through the existing `/api/tokens` admin API.

## Settings
- Testing: no
- Logging: minimal
- Docs: no

## Tasks

### Phase 1: Page markup

- [x] **Task 1: Build the static page shell**
  Files: `web/index.html`
  Replace the current placeholder body with the full page structure, all served verbatim by the task-11 embed file server (no framework, no bundler, no external assets). Include, top to bottom: a **token bar** with one text input (`id="grafana-token"`, `type="password"` acceptable) for the operator's Grafana token plus a "Connect" / reload button; a **create form** with a `name` text input (`id="new-name"`) and a **"Create"** button; a prominent **new-token panel** (initially hidden) to surface the plaintext value returned by `POST`; a **status/error line** (`id="status"`) for 401 re-prompts and toggle-failure messages; and a **table** (`<table id="tokens">` with a `<tbody>`) whose columns are Name, Token, Created, Active, Actions. Add minimal styling inline in `<head>` (`<style>`) — readable table, monospace token cells, visible destructive-button treatment for Delete. Load the script once with `<script src="app.js" defer></script>`. Keep the `<title>` and `lang="en"`.

### Phase 2: Client logic

- [x] **Task 2: Grafana token holding + authorized fetch helper** (depends on Task 1)
  Files: `web/app.js`
  Implement client-side token handling and a single `api(method, path, body)` helper used by every call. The Grafana token lives in memory (a module-scoped variable), optionally mirrored to `sessionStorage` (never `localStorage`, never a cookie, never written into page markup) and rehydrated on load so a same-tab reload keeps it. `api()` reads the current token, sends `Authorization: Bearer <token>`, and sets `Content-Type: application/json` for bodied requests.
  **Body parsing (critical — the API mixes JSON and empty responses):** decide the branch on `res.status`/`res.ok` **before** touching the body. Only `GET` (JSON array) and `POST` (`201` token object) return JSON; `PATCH` returns `200` with an **empty body** and `DELETE` returns `204` with **no body**, and every error path returns **plain text** (`http.Error`), not JSON. So never call `res.json()` unconditionally — parse JSON only when the response is OK **and** actually carries a body (e.g. `status !== 204` and non-empty `res.text()`), otherwise return `null`/void. Parsing empty PATCH/DELETE responses as JSON would throw and be misread as a failure — which for the toggle wrongly triggers Task 4's revert on a change the server accepted.
  **Status handling (distinguish auth failure from Grafana being down):** the admin gate returns `401` for a missing/invalid Grafana token but `503` ("grafana unreachable") when Grafana itself can't be reached (and other 5xx on store errors). Centralize this: on **`401`** clear/flag the held token, show a "re-enter Grafana token" message in the status line, focus the token input, and do **not** render stale rows; on **any other non-OK status (5xx / network error)** show the error in the status line but **keep** the held token (a valid token must not be wiped just because Grafana is momentarily down). Never `console.log` token values and never place them in URLs/query strings.

- [x] **Task 3: Load + render the token list** (depends on Task 2)
  Files: `web/app.js`
  On page load (if a token is present) and after every successful mutation, call `GET /api/tokens` and rebuild the table `<tbody>`. A rehydrated-but-now-invalid `sessionStorage` token must flow through the same `401` path (clear + re-prompt via the Task-2 helper), not render an empty/broken grid. Per returned row render: `name`; the plaintext `token` in a selectable/monospace cell with a small copy affordance; `created_at` formatted from the epoch value into a readable local timestamp; a **live `active` checkbox** whose `checked` reflects the row value and whose `onchange` is wired in Task 4; and a **Delete** button wired in Task 4. The copy affordance must guard `navigator.clipboard?` — it is `undefined` on plain `http://` non-localhost origins (the shared-server deployment) — and degrade gracefully (the selectable cell is the fallback) instead of throwing; never log the copied value. Wire the token bar's "Connect"/reload button to trigger this load. Build rows via DOM APIs (`createElement`/`textContent`), not string HTML interpolation of token values.

- [x] **Task 4: Create, live toggle, and delete actions** (depends on Task 3)
  Files: `web/app.js`
  Wire the three mutations, then refresh the list:
  - **Create:** the "Create" button POSTs `{ "name": <new-name value> }` to `/api/tokens`; on `201` surface the returned `token` prominently in the new-token panel for copying (the readable-plaintext moment), clear the name input, and reload the list.
  - **Live toggle (no save):** the row checkbox's `onchange` fires `PATCH /api/tokens/<id>` with `{ "active": <new checked value> }` immediately. On success leave the checkbox in its new state (optionally re-fetch to stay canonical). On failure (non-2xx or network error) **revert the checkbox to its prior state** and show the failure in the status line — never claim an `active` change the server rejected. There is no save button for the toggle.
  - **Delete (permanent):** the per-row "Delete" button prompts `confirm()`; on OK it fires `DELETE /api/tokens/<id>` (expect `204`) and reloads the list. Delete stays distinct from the toggle — an explicit, confirmed destructive action.
  Route all three through the Task-2 `api()` helper so `401` re-prompting is uniform.
