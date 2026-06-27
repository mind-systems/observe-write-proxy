# 12 — Token management page

**Task:** ROADMAP → Phase 6 — Admin GUI → "Token management page"
**Depth:** medium

## Goal

A vanilla, build-step-free GUI page that lets a single operator paste their Grafana token, see the proxy's write tokens with their plaintext values, flip a write token's `active` state live with no save step, and create new write tokens — all driven by the existing `/api/tokens` admin API.

## Design

- **Files, no framework, no build:** `web/index.html` plus a small vanilla `web/app.js`. Styling is minimal — inline in the HTML `<head>` or a tiny `web/style.css`. No bundler, no dependencies; the files are served verbatim by the embed-based file server (note 11).
- **Grafana token field:** a single text input for the operator's Grafana token, held **client-side only** — in memory, optionally mirrored to `sessionStorage` so a reload within the tab keeps it. It is **never** sent anywhere except as `Authorization: Bearer <token>` on every `/api/tokens` request, and is **never** persisted to disk by the proxy. There is no login endpoint; the token is validated by the API per request against Grafana.
- **Token list:** on load (and after each mutation) call `GET /api/tokens` and render one row per token. Each row shows: `name` (display label), the plaintext `token` value rendered so it can be selected/copied (a copy affordance is fine), `created_at`, and a **live `active` checkbox** bound to the row's current `active` value.
  - **Live toggle, no save:** the checkbox's `onchange` handler fires `PATCH /api/tokens/<id>` with body `{ "active": <new value> }` **immediately**. There are **no save buttons** anywhere on the page. The list reflects the server's truth: a successful PATCH leaves the checkbox in its new state (optionally re-fetch to stay canonical).
- **Create form:** a `name` text field plus a single **"Create"** button that POSTs `{ "name": <value> }` to `/api/tokens`. On success, surface the returned token value prominently so the operator can copy it (this is the readable-plaintext moment), then refresh the list. "Create" is the **only** action button on the page, and it creates — it is not a save.
- All four calls (`GET`, `POST`, `PATCH`) carry the `Authorization: Bearer <grafana token>` header pulled from the client-side field.

## Edge cases / watch

- **401 handling:** if any `/api/tokens` call returns `401` (missing/invalid Grafana token), prompt the operator to (re-)enter the Grafana token and do not render stale data as if authorized. Clear or flag the held token so the next attempt re-validates.
- **Failed toggle revert:** if the `PATCH` fails (non-2xx or network error), revert the checkbox to its prior state and signal the failure, so the UI never claims an `active` change that the server rejected. Because the toggle is the action, there is no save step to retry — the operator just toggles again.
- **Never persist the Grafana token to disk:** `sessionStorage`/in-memory only; never `localStorage`, never a cookie, never echoed back into the page markup that could be cached.
- **Plaintext token display is intentional** per canon (self-hosted, single-operator), but the page must avoid logging token values to the console or embedding them in URLs/query strings.
- **Deactivate, not delete:** the checkbox is the revoke control. There is no delete action — unchecking `active` is how a token is revoked, and re-checking reactivates it. Do not add a delete button.

## Out of scope

- Serving/embedding these files into the binary — covered by note 11.
- The `/api/tokens` handlers, Grafana validation, and the SQLite store behind them — owned by `internal/admin`, `internal/grafana`, `internal/store`.
- Any pagination, search, or filtering of the token list; the operator set is small.

## Done when

An operator opens `/`, pastes a Grafana token, and sees the list of write tokens with name, plaintext value, created time, and a live `active` checkbox; toggling a checkbox takes effect immediately via `PATCH` with no save step (and reverts visibly on failure); creating a token via the single "Create" button shows the new token value for copying and refreshes the list; and an invalid/expired Grafana token (`401`) re-prompts for the token instead of showing stale data.
