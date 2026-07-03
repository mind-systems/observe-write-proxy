# Plan: PATCH active must reject a missing field

## Context
Make `PATCH /api/tokens/<id>` reject a request body that omits `active` with `400`, instead of silently deactivating the token because a plain `bool` field defaults to `false`.

## Settings
- Testing: no
- Logging: minimal
- Docs: no

## Tasks

### Phase 1: Require the active field

- [x] **Task 1: Reject an absent `active` in the PATCH branch**
  Files: `internal/admin/tokens.go`
  In `TokenHandlers.Item`, PATCH branch (around lines 121-138), change the decode target from `struct{ Active bool `json:"active"` }` to `struct{ Active *bool `json:"active"` }`. Keep the existing malformed-JSON decode-error branch returning `400`. After a successful decode, add a nil check: if `body.Active == nil`, respond `http.Error(w, "active field is required", http.StatusBadRequest)` and return. Otherwise call `h.store.SetActive(id, *body.Active)`, leaving the `store.ErrNotFound` → `404`, internal-error → `500`, and success → `200` paths unchanged. Do not touch the DELETE branch or the `Collection` handler.
