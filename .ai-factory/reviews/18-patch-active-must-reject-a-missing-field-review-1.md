# Code Review: PATCH active must reject a missing field

**Files reviewed:** `internal/admin/tokens.go` (full file), verified against the plan and spec note `.ai-factory/notes/18-patch-active-required.md`.
**Risk level:** 🟢 Low

## Change

`TokenHandlers.Item` PATCH branch decode target changed from `struct{ Active bool }` to `struct{ Active *bool }`, plus a `body.Active == nil` → `400 "active field is required"` guard added after a successful decode, and `SetActive` now receives `*body.Active`.

## Correctness verification

- **Root cause fixed.** With the pointer, `{}` and any body lacking `active` decode to `nil` and now return `400` with no state change, instead of decoding to `false` and silently deactivating with `200`.
- **Present-and-false still works.** `{"active":false}` decodes to a non-nil `&false`, passes the nil guard, and calls `SetActive(id, false)` — deactivation is preserved. `{"active":true}` reactivates. Both remain `200`.
- **Branch ordering correct.** Malformed JSON still hits the existing `Decode` error → `400` branch (lines 125-128) before the new nil check runs, so it is unaffected. The nil guard only executes after a successful decode.
- **Downstream paths untouched.** `store.ErrNotFound` → `404`, internal error → `500`, success → `200` all preserved. `*body.Active` is a `bool`, matching `SetActive(id string, active bool)`. No nil-deref risk — the pointer is guaranteed non-nil past line 132.
- **Scope respected.** DELETE branch, `Collection` handler, and the store are untouched. No schema/migration involved.
- **Build:** `go build ./...` passes. No test files in `internal/admin/` reference the old behavior.

## Findings

None.

REVIEW_PASS
