# Code Review: Management endpoints (10)

**Scope:** `internal/admin/tokens.go` (new), `cmd/proxy/main.go` (mux wiring).
**Build:** `go build ./...` and `go vet ./...` both clean.
**Verdict:** No blocking findings. Implementation is correct, matches the plan and roadmap contract, and respects the two-plane / dependency rules.

## What was checked

- **Routing (Go 1.19 mux, no wildcards).** `/api/tokens` (exact) → `Collection`, `/api/tokens/` (subtree) → `Item`. Both are registered explicitly, so a request to `/api/tokens` hits the exact handler and there is **no** subtree 301 redirect surprise. Token ids are `base64.RawURLEncoding` (16 bytes) — never contain `/` — so `strings.TrimPrefix(r.URL.Path, "/api/tokens/")` yields a clean single-segment id. Registration order and the `/`, `/healthz`, `/v1/logs` routes are untouched.
- **Status codes vs contract.** POST→`201` with full `tokenResponse` incl. plaintext `token`; GET→`200` array; PATCH→`200`; DELETE→`204` no body; collection non-GET/POST (incl. `DELETE`, `PATCH`)→`405` with `Allow: GET, POST`; item non-PATCH/DELETE→`405` with `Allow: PATCH, DELETE`; bad JSON→`400`; unknown id (`store.ErrNotFound` via `errors.Is`)→`404`; other store errors→`500`. All match the roadmap guards exactly.
- **Empty list encodes as `[]` not `null`** — `make([]tokenResponse, len(tokens))` guarantees a non-nil slice. Correct.
- **Empty id** (`/api/tokens/`) → `404` before the method switch. Correct.
- **Dependencies / planes.** `admin → store` import is the allowed edge; no `admin → proxy`, no cycle (`store` imports nothing local). Narrow `tokenStore` interface mirrors the existing `grafanaValidator` / `tokenLookup` pattern. Handlers sit behind the unchanged Grafana `gate`; write plane untouched. `*store.Store` satisfies `tokenStore`.
- **Security.** Plaintext token in the response is intentional per canon (read-back in GUI); token values are not logged. No auth-plane mixing. `store` uses parameterized SQL, so the trailing-path id is not an injection vector.
- **Concurrency.** `*sql.DB` is safe for concurrent use (store opened with WAL + busy_timeout in task 05); handlers hold no shared mutable state.

## Minor, non-blocking observations

1. **`PATCH` with a JSON body that omits `active` silently deactivates the token.** `body.Active` is a non-pointer `bool`, so a valid but field-less body — `{}`, or a typo'd key like `{"actve":true}` — decodes to `Active=false` and calls `SetActive(id, false)` with a `200`. Go JSON cannot distinguish "absent" from "false" without a `*bool`. The task-12 GUI always sends `{active:<bool>}`, so this does not affect the intended client, but the endpoint is slightly permissive for other callers. Optional hardening: use `*bool` and `400` when nil. Not required by the spec (which explicitly allowed a minimal `200`).
2. **Empty `name` accepted on `POST`.** `store.Create("")` inserts a row (`name TEXT NOT NULL` is satisfied by `""`). Acceptable per canon — `name` is a display-only label with no authorization meaning — but a blank-name `400` would be friendlier UX. Not required.
3. **Empty/EOF request body on POST/PATCH → `400`.** `json.Decode` returns `io.EOF`, caught by the malformed-body branch. This is the correct and intended behavior; noted only for completeness.

None of the above changes runtime correctness against the documented contract or the GUI that consumes this API.

REVIEW_PASS
