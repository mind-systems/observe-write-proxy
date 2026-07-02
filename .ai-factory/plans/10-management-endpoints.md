# Plan: Management endpoints

## Context
Add the Grafana-gated JSON management API (`internal/admin`) that mints, lists, toggles, and deletes write-plane tokens by calling the existing `store` operations, replacing the `handleNotImplemented` placeholders behind the task-09 gate.

## Settings
- Testing: no
- Logging: minimal
- Docs: no

## Tasks

### Phase 1: Admin token API

- [x] **Task 1: Add response DTO, narrow store interface, and JSON helpers**
  Files: `internal/admin/tokens.go`
  Create the new file in package `admin`. Define a narrow `tokenStore` interface exposing only what these handlers use — `Create(name string) (store.Token, error)`, `List() ([]store.Token, error)`, `SetActive(id string, active bool) error`, `Delete(id string) error` — mirroring the `grafanaValidator` narrow-interface pattern already in `internal/admin/auth.go` (depend on the shape used, not on `*store.Store`'s full surface). Define a `TokenHandlers` struct holding a `tokenStore` and a `NewTokenHandlers(s tokenStore) *TokenHandlers` constructor. Add a JSON response DTO `tokenResponse{ID string \`json:"id"\`; Name string \`json:"name"\`; Token string \`json:"token"\`; Active bool \`json:"active"\`; CreatedAt int64 \`json:"created_at"\`}` and a small `func newTokenResponse(t store.Token) tokenResponse` mapper (`store.Token` has fields `ID, Name, Token, Active, CreatedAt`). Add a `writeJSON(w, status, payload)` helper that sets `Content-Type: application/json`, calls `w.WriteHeader(status)`, and encodes with `encoding/json`. Follow error-at-the-HTTP-boundary conventions from `internal/proxy/handler.go` (`http.Error` with plain messages). Do not log token values (base rule).

- [x] **Task 2: Implement the collection handler (`/api/tokens`)** (depends on Task 1)
  Files: `internal/admin/tokens.go`
  Add method `func (h *TokenHandlers) Collection(w http.ResponseWriter, r *http.Request)` switching on `r.Method`:
  - `GET`: call `List()`; on store error → `500`; on success write `200` with a JSON array of `tokenResponse` (emit `[]`, not `null`, when empty).
  - `POST`: decode the request body into `struct{ Name string \`json:"name"\` }`; malformed/undecodable JSON → `400` (guard); call `Create(name)`; on store error → `500`; on success write `201` with the full `tokenResponse` including the plaintext `token` (returned once to copy).
  - Any other method (notably `DELETE`, `PATCH`): set `Allow: GET, POST` header and return `405` (guard: the collection rejects `DELETE`).
  Match the method-guard style of `proxy.Handler.ServeHTTP` (sets `Allow`, returns `405`).

- [x] **Task 3: Implement the item handler (`/api/tokens/<id>`)** (depends on Task 1)
  Files: `internal/admin/tokens.go`
  Add method `func (h *TokenHandlers) Item(w http.ResponseWriter, r *http.Request)`. Parse the trailing id with `strings.TrimPrefix(r.URL.Path, "/api/tokens/")`; an empty id (bare `/api/tokens/`) → `404`. Switch on `r.Method`:
  - `PATCH`: decode `struct{ Active bool \`json:"active"\` }`; malformed JSON → `400`; call `SetActive(id, active)`; map `store.ErrNotFound` (use `errors.Is`) → `404`, other store error → `500`; on success write `200` (return the updated `tokenResponse` or an empty `200` — keep it minimal, but stay consistent).
  - `DELETE`: call `Delete(id)`; `store.ErrNotFound` → `404`, other error → `500`; on success `204` with no body.
  - Any other method: set `Allow: PATCH, DELETE` and return `405`.
  Note the `store.ErrNotFound` sentinel already exists in `internal/store/token.go`.

- [x] **Task 4: Wire the handlers into the mux behind the task-09 gate** (depends on Task 2, Task 3)
  Files: `cmd/proxy/main.go`
  In `newMux`, replace the two `handleNotImplemented` placeholders for `/api/tokens` and `/api/tokens/` with the real admin handlers, keeping the `gate(...)` wrapper intact so both stay behind the Grafana admin-auth middleware. Construct the handlers via `admin.NewTokenHandlers(st)` (pass the existing `*store.Store`, which satisfies the narrow `tokenStore` interface) and register `gate(http.HandlerFunc(h.Collection))` on `/api/tokens` and `gate(http.HandlerFunc(h.Item))` on `/api/tokens/`. Thread the handlers into `newMux` (extend its signature or build them inside `main` and pass in) without disturbing the `/`, `/healthz`, or `/v1/logs` registrations or their ordering. Update the `newMux` doc comment to reflect that `/api/tokens[/]` now serve the real management API instead of placeholders.
