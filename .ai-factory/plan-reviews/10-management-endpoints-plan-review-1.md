# Plan Review: Management endpoints (10)

**Plan:** `.ai-factory/plans/10-management-endpoints.md`
**Risk Level:** 🟢 Low
**Verdict:** Solid — accurate against the codebase, no blocking issues.

## Verification against the codebase

Every concrete claim in the plan was checked against the actual source:

- **Store method signatures** — `Create(name string) (Token, error)`, `List() ([]Token, error)`, `SetActive(id string, active bool) error`, `Delete(id string) error` all match `internal/store/token.go` exactly. `*store.Store` therefore satisfies the proposed narrow `tokenStore` interface.
- **`store.Token` fields** — `ID, Name, Token, Active, CreatedAt` (with `CreatedAt int64`) confirmed; the `tokenResponse` DTO and `newTokenResponse` mapper align.
- **`store.ErrNotFound`** — exists in `internal/store/token.go` and is returned **only** by `SetActive` and `Delete`. The plan's mapping (ErrNotFound → 404 for PATCH/DELETE; other store errors → 500; List/Create errors → 500) is correct.
- **Narrow-interface pattern** — `grafanaValidator` in `internal/admin/auth.go` is exactly the precedent cited; mirroring it is consistent.
- **Method-guard style** — `proxy.Handler.ServeHTTP` sets `Allow` + returns 405 as claimed; `http.Error` with plain messages is the boundary convention.
- **Mux wiring** — `newMux` in `cmd/proxy/main.go` currently registers `/api/tokens` and `/api/tokens/` as `gate(http.HandlerFunc(handleNotImplemented))`; replacing both while keeping `gate(...)` intact is straightforward. `st` remains needed (line 103 `proxy.NewAuthMiddleware(st, h)`), so threading admin handlers in as extra params is clean.
- **Go 1.19 mux semantics** — `go.mod` confirms `go 1.19`, so there are no path wildcards. Registering exact `/api/tokens` (Collection) alongside subtree `/api/tokens/` (Item) with in-handler method switching is the correct approach; both being registered avoids the subtree-redirect surprise. Token IDs are `base64.RawURLEncoding` (no `/`), so `strings.TrimPrefix(path, "/api/tokens/")` yields a clean single-segment id.

## Context Gates

- **Architecture (`.ai-factory/ARCHITECTURE.md`):** PASS. Dependency rules state `admin/ → store/ + grafana/`; the plan adds an `admin → store` import, which is explicitly allowed. No forbidden edge (admin never imports proxy; store imports nothing) is introduced. No import cycle (store does not import admin).
- **Rules (`.ai-factory/RULES.md`):** Not present — WARN (optional file absent, non-blocking).
- **Roadmap (`.ai-factory/ROADMAP.md`):** PASS. The plan implements the open **"Management endpoints"** task verbatim — endpoints, status codes (`201`/`200`/`204`/`405`/`400`/`404`), plaintext token returned once, list including `token` + `active`, and the "collection rejects DELETE → 405, bad JSON → 400, unknown id → 404" guards all match. The task's spec note (`.ai-factory/notes/10-token-management-endpoints.md`) is referenced in the roadmap but not linked from the plan; non-blocking, but citing it would help the implementer.
- **Migrations:** None required and none omitted — the `tokens` table already carries every field these handlers touch. Correctly silent.
- **Security:** Endpoints sit behind the existing Grafana `gate`; "do not log token values" is called out; returning plaintext tokens on create/list is intentional per canon (plaintext-at-rest, read-back in GUI). No new auth-plane mixing.

## Minor observations (non-blocking)

1. **Empty `name` on `POST /api/tokens`** — the plan does not validate a missing/empty name. `store.Create` accepts `""` (`name TEXT NOT NULL` is satisfied by an empty string), and the canon treats `name` as display-only with no authorization meaning, so this is acceptable. If the implementer wants nicer UX, a `400` on blank name is a reasonable option — but not required.
2. **Empty request body on PATCH/POST** — a `json.Decoder` on an empty body returns `io.EOF`, which the "malformed JSON → 400" rule already covers. Worth the implementer treating EOF as a 400 rather than a 500; the plan's wording is consistent with that.
3. **Encode failure after `WriteHeader`** — `writeJSON` sets status before encoding, so a mid-encode error can't change the already-sent status. This is the standard Go limitation and fine for these small payloads; no action needed.

## Positive Notes

- Correctly anchors on the Go 1.19 constraint and avoids introducing a router dependency.
- Reuses the established narrow-interface and error-at-the-boundary conventions rather than inventing new ones.
- Dependency direction and the two-plane split are respected; no architectural boundary is crossed.
- Task decomposition is coherent with clean dependencies (Task 1 → 2/3 → 4) and stays entirely within the two files it names.

PLAN_REVIEW_PASS
