# 18 — PATCH active must reject a missing field

**Task:** ROADMAP → Phase 10 — Post-review fixes → "PATCH active must reject a missing field"
**Depth:** low

## Goal

`PATCH /api/tokens/<id>` treats an omitted `active` field as a client error (`400`) instead of silently deactivating the token. Today the handler decodes into `struct{ Active bool }`, so a body like `{}` or `{"foo":1}` yields `Active == false` (Go's zero value) and the token is deactivated with a `200` — a state change the caller never asked for.

## Design

- In `internal/admin/tokens.go`, `Item`'s `PATCH` branch, change the decode target's field to a pointer: `struct{ Active *bool `json:"active"` }`.
- After a successful decode, if the pointer is nil (field absent), respond `400` ("active field is required") and return.
- Otherwise call `store.SetActive(id, *body.Active)` as before; the `ErrNotFound` → `404` and success → `200` paths are unchanged.
- Malformed JSON still returns `400` from the existing decode-error branch.

## Edge cases / watch

- A pointer distinguishes "field absent" (nil → `400`) from "field present and false" (`&false` → deactivate) — a plain `bool` cannot. This is the whole point of the change.
- `{"active":false}` and `{"active":true}` must still work exactly as before — only the *absent* field becomes an error.
- Leave the live-toggle GUI path untouched: `web/app.js` always sends an explicit `active`, so it is unaffected.

## Done when

- `PATCH /api/tokens/<id>` with `{}` (or any body lacking `active`) returns `400` and changes nothing.
- `{"active":false}` then `{"active":true}` still deactivate and reactivate, each `200`.
- An unknown id still returns `404`; malformed JSON still `400`.
