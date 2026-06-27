# 10 — Management endpoints

**Task:** ROADMAP → Phase 5 — Grafana-authenticated admin API → "Management endpoints"
**Depth:** medium

## Goal

The JSON API the operator drives through the GUI to mint, list, and toggle write tokens. Sitting behind the task-09 Grafana auth middleware and backed by `internal/store`, it lets an operator create a named token (returned in plaintext so it can be copied), list existing tokens with their current state, and flip a token active or inactive — with no endpoint that ever deletes a token.

## Design

- Handlers live in `internal/admin`, registered behind the task-09 middleware, calling `internal/store` only (the store owns SQLite; `admin` does not touch the DB directly).
- **Routing under the Go 1.19 mux constraint.** `http.ServeMux` has no method or path wildcards (those land in Go 1.22), so the API splits into two registrations and no router dependency is added:
  - `/api/tokens` — the **collection** handler. Switch on `r.Method`: `POST` (create), `GET` (list); anything else → `405`.
  - `/api/tokens/` — the **item** handler (note the trailing slash so the mux routes the subtree here). Parse the trailing segment after `/api/tokens/` as the `<id>` manually; switch on `r.Method`: `PATCH` (set active); anything else → `405`. An empty/missing id segment → `404`.
- **`POST /api/tokens`** — decode body `{ "name": "..." }`. Generate the token value (the store or handler mints it; the canon does not place it in the request). Call `store.Create(name)` → respond `201` with the full row as JSON: `{ id, name, token, active, created_at }`. The **plaintext `token` is returned here on purpose** so the operator can copy it once at creation (and per canon it remains readable later via list).
- **`GET /api/tokens`** — call `store.List()` → respond `200` with a JSON array of rows, each including the **plaintext `token`** and `active`. Plaintext exposure is intentional per canon (self-hosted, single-operator stack); the GUI shows the value directly.
- **`PATCH /api/tokens/<id>`** — decode body `{ "active": true|false }`. Call `store.SetActive(id, active)` → respond `200`. This is the single deactivate/reactivate path: it is how a token is "revoked" (set inactive) and brought back (set active). **There is no `DELETE` handler and no delete path anywhere** — rows are never removed.
- Response bodies are JSON throughout; set `Content-Type: application/json`.

## Edge cases / watch

- **No DELETE, ever** — neither the collection nor the item handler accepts `DELETE`; it falls through to `405` like any other unsupported method. "Revoke" is `PATCH active:false`, not a deletion.
- **Method gating per route** — `405 Method Not Allowed` for unsupported verbs on each handler (e.g. `PATCH` on the collection, `POST` on the item).
- **Malformed JSON / wrong shape** → `400`. For `POST`, an empty or whitespace-only `name` is a `400`; `name` is display-only and never authorises anything, but an empty label defeats its only purpose (helping the operator recognise the token).
- **`PATCH` on an unknown id** → `404` — the store reports no row affected; the handler must surface that as not-found, not a silent `200`. (Decide via a "rows affected" / not-found signal from `store.SetActive`.)
- **Trailing-segment parsing** — strip the `/api/tokens/` prefix and reject anything with further path segments (e.g. `/api/tokens/5/extra`) as `404`; only a bare id is valid.
- **Never log the token value** — even on the admin plane, response logging must not echo the plaintext token (canon: never log token values).

## Out of scope

- The Grafana-token gate in front of these handlers (task 09) and the validation client it uses (task 08).
- The SQLite implementation of `Create` / `List` / `SetActive` and token generation internals (the `internal/store` task).
- The static GUI that calls this API and renders the live `active` checkbox (its own task); this note defines only the JSON contract it consumes.
- The write plane (`/v1/logs`) — a different plane entirely.

## Done when

- `POST /api/tokens` with `{ "name": "..." }` returns `201` and a body containing the new plaintext `token` plus `id`, `name`, `active`, `created_at`.
- `GET /api/tokens` returns the list of tokens including each plaintext `token` and `active` flag.
- `PATCH /api/tokens/<id>` with `{ "active": false }` then `{ "active": true }` deactivates and reactivates that token, each returning `200`.
- Unsupported methods → `405`, bad JSON → `400`, unknown id on `PATCH` → `404`, and no request to any endpoint deletes a token.
