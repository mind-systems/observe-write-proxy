# Code Review: Server-side name validation on create

**Scope:** `internal/admin/tokens.go` (the only source change). ROADMAP/notes/plan artifacts are documentation and out of scope for runtime review.

## Change under review

```go
if strings.TrimSpace(body.Name) == "" {
    http.Error(w, "name must not be empty", http.StatusBadRequest)
    return
}
```

Inserted in `Collection`'s `http.MethodPost` branch, after the JSON decode succeeds and before `h.store.Create(body.Name)`.

## Verification

- **Placement correct.** The guard sits after decode and before `Create`, so a rejected name never reaches the store — nothing is minted, matching the spec's "return 400 and mint nothing".
- **Trim covers whitespace.** `strings.TrimSpace` reduces `""`, `"   "`, tabs, and newlines to `""`, so both `{"name":""}` and `{"name":"   "}` return `400` as required.
- **Import present.** `strings` is already imported (`tokens.go:7`, used by `Item`); no import change, `go build` stays clean.
- **Status semantics preserved.** `400` (client error) is distinct from the `500` returned on a `store.Create` failure, keeping the client-vs-server error split intact.
- **Happy path unchanged.** A non-empty name (`{"name":"tradeoxy-broker"}`) skips the guard and still returns `201` with the created token.
- **No-echo discipline honored.** The error message is a static string; the request body is never logged.
- **No state/schema impact.** Pure request validation — no store, no DB, no migration; `http.Error` writes the `400` with no body side effects.

## Notes

- Notes 18 (PATCH `active` required) and 19 (verify.sh guidance) are separate Phase 10 tasks and are correctly **not** implemented here — this milestone is scoped to name validation only. No scope creep.
- `http.Error` sets `Content-Type: text/plain` while the GET/POST success paths use `writeJSON`. This mixed content type already exists throughout both handlers (all `http.Error` calls) and is consistent with the surrounding code — not a regression.

No correctness, security, or runtime concerns.

REVIEW_PASS
