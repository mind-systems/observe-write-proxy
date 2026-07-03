# 17 — Server-side name validation on create

**Task:** ROADMAP → Phase 10 — Post-review fixes → "Server-side name validation on create"
**Depth:** low

## Goal

`POST /api/tokens` rejects an empty or whitespace-only `name` with `400`, closing the gap between the shipped handler and the note-10 contract. Today the handler mints a token for `{"name":""}` and returns `201`; the display-only name is then a blank label that defeats its only purpose (letting the operator recognise which token to revoke).

## Design

- In `internal/admin/tokens.go`, `Collection`'s `POST` branch, after decoding the body and before calling `store.Create`: if `strings.TrimSpace(body.Name) == ""`, respond `400` with a short plain message ("name must not be empty") and return. `strings` is already imported.
- No store change — validation is a handler concern; `store.Create` stays a dumb insert.
- The GUI (`web/app.js`, `onCreate`) already blocks an empty name client-side, so this only affects direct API callers; behaviour through the GUI is unchanged.

## Edge cases / watch

- Trim before the check so a name of only spaces/tabs is also rejected — a whitespace label is as useless as an empty one.
- Keep it a `400` (client error), distinct from the `500` a `store.Create` failure returns.
- Do not log the request body — no secret here, but keep the handler's no-echo discipline.

## Done when

- `POST /api/tokens {"name":""}` and `{"name":"   "}` both return `400` and mint nothing.
- `POST /api/tokens {"name":"tradeoxy-broker"}` still returns `201` with the created token.
- The GUI create flow is unaffected.
