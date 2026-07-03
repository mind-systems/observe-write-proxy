# Plan: Server-side name validation on create

## Context
Reject an empty or whitespace-only `name` on `POST /api/tokens` with `400`, closing the direct-API gap where `{"name":""}` currently mints a token and returns `201`.

## Settings
- Testing: no
- Logging: minimal
- Docs: no

## Tasks

### Phase 1: Validation guard

- [x] **Task 1: Guard empty/whitespace name in the POST branch**
  Files: `internal/admin/tokens.go`
  In `Collection`'s `http.MethodPost` case, after the JSON decode succeeds and before the `h.store.Create(body.Name)` call, add: `if strings.TrimSpace(body.Name) == "" { http.Error(w, "name must not be empty", http.StatusBadRequest); return }`. `strings` is already imported (used in `Item`), so no import change. Do not touch `store.Create` — validation stays a handler concern. Do not log the request body, matching the existing no-echo discipline. This leaves `{"name":"tradeoxy-broker"}` returning `201` unchanged and makes both `{"name":""}` and `{"name":"   "}` return `400` with nothing minted.
