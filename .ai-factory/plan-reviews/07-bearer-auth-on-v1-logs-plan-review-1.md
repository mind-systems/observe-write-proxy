# Plan Review: 07-bearer-auth-on-v1-logs

## Code Review Summary

**Files Reviewed:** plan + `internal/store/{store.go,token.go}`, `internal/proxy/{handler.go,forwarder.go}`, `cmd/proxy/main.go`, `internal/config/config.go`, `.ai-factory/ARCHITECTURE.md`, `.ai-factory/rules/base.md`
**Risk Level:** 🟢 Low

### Context Gates

- **Architecture (PASS):** The plan honors the frozen dependency rules. `proxy/ → store/ only` is exactly what the `tokenLookup` interface + `store` import establishes; nothing pulls in `admin/` or `grafana/`. The write-plane/admin-plane split is preserved — this task gates only `/v1/logs` and leaves `/api/tokens*` and `/` for the Grafana-token plane. The label-blind guarantee is explicitly upheld ("do not read, buffer, or parse `r.Body`"). Request flow matches ARCHITECTURE.md's "token must exist **and** be active → forward; unknown or inactive → 401".
- **Rules (PASS):** Naming (`snake_case.go` file `auth.go`, unexported `tokenLookup`, `PascalCase` `NewAuthMiddleware`), error-at-boundary policy (`401` invalid token, `5xx` for backend failure), and the "no token values / no payloads in logs" logging rule are all respected.
- **Roadmap (WARN — non-blocking):** No `.ai-factory/skill-context/aif-review/SKILL.md` exists, so no project-specific overrides apply. Linkage to a ROADMAP milestone is not stated in the plan header; optional for this repo's plan format (prior plans 01–06 follow the same shape), so no action required.

### Correctness — verified against the codebase

- **Signatures match exactly.** `store.Lookup(token string) (Token, bool, error)` (token.go:101), `store.Open(path) (*Store, error)` (store.go:44), `(*Store).Close()` (store.go:72), and `proxy.NewHandler(fwd) *Handler` (handler.go:18) all match what the plan calls. The `tokenLookup` interface method signature is a precise fit, and `*store.Store` satisfies it.
- **`*Handler` is already `http.Handler`** (handler.go implements `ServeHTTP`), so wrapping it with `NewAuthMiddleware(lookup, next http.Handler)` without editing `handler.go` is sound — handler.go's own doc comment anticipates exactly this ("so a later task can wrap it with auth middleware without editing it").
- **Three-way `Lookup` result interpretation is correct.** `ok==false, err==nil` (unknown *or* inactive, since `active=1` is filtered in SQL — token.go:107) → `401`; `err!=nil` → `500`; valid → `next`. This is the right mapping and avoids the classic "store error masquerades as auth failure" bug.
- **No migration needed and none proposed — correct.** The `tokens` table and `active` column already exist (store.go:21–27) and `Lookup` already enforces `active=1`; this task is pure read-path gating, so there is no schema change to make.
- **No auth-token leak to Loki.** `Forwarder.Forward` builds a fresh request and copies only `Content-Type`/`Content-Encoding` (forwarder.go:82–85); the inbound `Authorization` header is never propagated upstream even though the middleware passes the original `*http.Request` to `next`. The plan need not strip it — behavior is already safe.

### Observations (non-blocking, implementer discretion)

- **Task 2 wording is slightly redundant:** it says "construct the forwarder handler as today (`proxy.NewHandler(fwd)`), then `authed := proxy.NewAuthMiddleware(st, proxy.NewHandler(fwd))`". That phrasing implies two `NewHandler` calls; intent is clearly one handler wrapped once. A clean read: `authed := proxy.NewAuthMiddleware(st, proxy.NewHandler(fwd))` and `mux.Handle("/v1/logs", authed)`. No correctness impact.
- **`newMux` signature change ripples to one call site only** (main.go:33), and the plan says so. Passing the already-wrapped `http.Handler` into `newMux` (rather than the store) keeps `newMux` free of a `store` import and is marginally cleaner; either variant offered by the plan compiles.
- **Store lifetime on shutdown:** the plan's `defer st.Close()` runs at `main` return, i.e. after `srv.Shutdown` — correct ordering, no in-flight lookup can race the close. Fine as written.
- **Method-vs-auth ordering:** with the middleware outermost, a `GET /v1/logs` without a token now yields `401` rather than the handler's `405`. This is acceptable (auth precedes method) and arguably preferable; just noting the behavior change so it isn't mistaken for a bug.

### Positive Notes

- Explicitly rejects a false requirement (constant-time compare) with a correct rationale — `Lookup` is a keyed SQL query, not a byte-compare handler, and the stack is single-operator.
- No-oracle discipline (identical `401` status/body/headers for malformed-header and unknown/inactive-token cases) is called out precisely.
- Header parsing spec is robust: case-insensitive `bearer` scheme match, trim, and rejection of empty-after-trim credentials.
- The plan reuses existing, verified primitives rather than duplicating logic, and stays inside the architectural dependency envelope.

PLAN_REVIEW_PASS
