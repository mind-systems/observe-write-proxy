# Code Review: /v1/logs route (04) — review 1

## Scope

Changed code files:
- `internal/proxy/handler.go` (new) — the `POST /v1/logs` handler.
- `cmd/proxy/main.go` (modified) — construct the forwarder, wire the handler into the mux.

Reviewed in full against `internal/proxy/forwarder.go`, `internal/config/config.go`, the plan `04-v1-logs-route.md`, the spec note `.ai-factory/notes/04-write-logs-route.md`, and ARCHITECTURE.md. `go build ./...` and `go vet ./...` both pass.

## Assessment

The change is small, correct, and matches the spec end-to-end.

- **Method guard** runs before the forwarder is touched: non-POST → `405` with `Allow: POST`, so a GET never reaches Loki (spec "Done when" #2). ✓
- **Content headers** are read with `r.Header.Get` and passed through unvalidated/undefaulted — content-agnostic, label-blind. ✓
- **Streaming**: `r.Body` is passed directly to `Forward`; the body is never read, buffered, or parsed in the handler. Preserves the streaming + label-blind guarantee. ✓
- **Error path**: on a non-nil `Forward` error the handler writes `StatusForError(err)` (502/504) — never a 200 for a failed forward. `resp` is nil on that path (forwarder.go:88–90), and `defer resp.Body.Close()` is correctly placed *after* the error check, so there is no nil-pointer dereference. ✓
- **Success relay**: `Content-Type` is copied, then `WriteHeader(resp.StatusCode)`, then `io.Copy` — correct ordering (no header set after `WriteHeader`), and Loki's 2xx/4xx/5xx are relayed verbatim without inventing a status. `resp.Body` is closed via `defer`. ✓
- **Wiring**: `main` fails fast on `NewForwarder` error, matching the existing config-load pattern; `newMux` takes the handler and mounts it with `mux.Handle`. `/api/tokens`, `/api/tokens/`, `/` remain placeholders. ✓
- **Middleware-ready shape**: `Handler` owns only `*Forwarder` and exposes a plain `ServeHTTP`, so task 07 can wrap it as middleware without editing this file (spec goal). ✓
- **Dependency rules**: the handler imports only `io`/`net/http` and uses the in-package `Forwarder`/`StatusForError`; it pulls in neither `admin/` nor `grafana/`. ✓

## Notes (non-blocking, no action required)

These are consistent with the plan's explicit "Logging: minimal" setting and the forwarder's design — recording them so they are understood as deliberate, not overlooked:

1. **No server-side log on forward failure.** A down/refusing Loki yields a 502/504 to the client but leaves no operator signal. Deferred by the minimal-logging setting; a single non-sensitive log line (status + wrapped error, never body/token) could be added later.
2. **`io.Copy` error is intentionally unhandled.** Once `WriteHeader` is sent the status is fixed, so a mid-body copy failure can only be logged, not corrected — silent discard is the standard choice.
3. **Response `Content-Encoding` is not relayed.** Correct: `Forward` sets no `Accept-Encoding`, so Go's transport transparently requests+decompresses gzip and strips `Content-Encoding` from `resp` — there is normally nothing to relay, and copying it blindly would misrepresent the (now-decompressed) body.

No correctness, security, or runtime defects found.

REVIEW_PASS
