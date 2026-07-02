# Plan Review 1: 03 — Loki forwarder

## Code Review Summary

**Files Reviewed:** plan `plans/03-loki-forwarder.md` against spec note `notes/03-loki-forwarder.md`, `internal/config/config.go`, `cmd/proxy/main.go`, `ARCHITECTURE.md`, `ROADMAP.md`, `go.mod` (Go 1.19), and prior review `plan-reviews/02-http-server-lifecycle-plan-review-2.md`.
**Risk Level:** 🟢 Low — the plan faithfully implements the spec note and honors the architectural spine. No critical issues; a few non-blocking notes, one of which is a carry-over the prior review explicitly deferred to this task.

### Context Gates

- **Architecture (`.ai-factory/ARCHITECTURE.md`):** ✅ PASS. The plan creates `internal/proxy` as the write plane, standard-library-only, and explicitly forbids importing `internal/admin`/`internal/grafana` — matching the dependency rule "`proxy/` never imports `admin/` or `grafana/`". The label-blind guard ("never read, buffer, decompress, or parse the body") maps 1:1 to "❌ no package parses or rewrites the forwarded OTLP body." The `<loki>/otlp/v1/logs` join is done here in `proxy`, exactly where config deferred it (`config.go:19-21`). Task 1 correctly notes no `store` use yet, consistent with the note.
- **Rules:** `.ai-factory/RULES.md` is absent (empty); a `.ai-factory/rules/` dir exists (`rules/base.md` was cited by prior reviews). No conflict — plan keeps package names lowercase, adds no dependency (cgo-free invariant preserved), keeps planes separated, and logs no token/payload. ✅ PASS (WARN: no top-level `RULES.md`, non-blocking).
- **Roadmap (`.ai-factory/ROADMAP.md`):** ✅ PASS. Maps 1:1 to Phase 2 → "Loki forwarder." Scope (forwarder only, no route, no auth) matches the milestone text and its "no body buffering, no hop-by-hop headers" guard. Route wiring is correctly deferred to the "/v1/logs route" milestone (task 04).

### Codebase assumptions — verified

- `internal/proxy` does not exist yet — plan creates it, consistent with "package `proxy` does not exist." ✅
- Module path `github.com/mind-systems/observe-write-proxy`, Go 1.19 (`go.mod`). All named APIs — `http.NewRequestWithContext`, `http.Client{Timeout}`, `errors.Is`, `os.IsTimeout`, `context`, `url.Parse` — exist in Go 1.19. ✅
- Config exposes `LokiURL` as a *base* URL, deliberately not the `/otlp/v1/logs` endpoint (`config.go:19-21`); the plan's `NewForwarder(lokiBaseURL)` + append is the intended consumer. ✅
- `config.go` already validates the base URL via `isValidBaseURL` (Scheme + Host). The plan's re-validation in `NewForwarder` is defensive duplication, not a contradiction — reasonable for a package that shouldn't trust silently. The sentinel-error + `fmt.Errorf("proxy: …: %w", …)` pattern mirrors `config.go:43-48`. ✅
- The `main.go` route table already has a `/v1/logs` `501` placeholder (`main.go:76`); nothing in this task wires the forwarder in, which is correct — the forwarder is a library package with exported symbols and no caller until task 04. Go compiles this fine. ✅

### Failure-classification correctness — confirmed

The plan classifies timeouts with **both** `errors.Is(err, context.DeadlineExceeded)` **and** `os.IsTimeout` → 504, everything else → 502. This dual check is correct and necessary:
- A caller `ctx` deadline surfaces as `context.DeadlineExceeded` → caught by `errors.Is`.
- The `http.Client.Timeout` expiry does **not** wrap `context.DeadlineExceeded` (it's an internal "Client.Timeout exceeded" error) — but the returned `*url.Error` implements `Timeout() bool`, so `os.IsTimeout` catches it.

Neither check alone covers both cases; using both is the right call. "No retries" is correctly justified by non-idempotent log ingestion.

### Critical Issues

None.

### Non-blocking Notes

- **Forwarder client timeout (30s) equals the server `WriteTimeout` (30s) — carry-over from review 2, Note #4, still unaddressed.** `main.go:33` sets `WriteTimeout: 30 * time.Second`, which bounds the whole handler (request-header-read → response-write). The plan again picks a 30s client timeout. With zero margin, a legitimate ~29s Loki round-trip can race the server envelope: the forwarder may just complete while the server cuts the response write at the same instant, or the forwarder's own 30s expires before the server can render a clean 504. Prior review flagged this to be handled *here*. Recommend the forwarder timeout be **strictly less** than the server `WriteTimeout` (e.g. 25s), or that the const be chosen with that relationship documented. Not a blocker, but it's the one item the plan was expected to close.
- **`http.Client.Timeout` keeps running while the caller reads `resp.Body`.** Because the plan (correctly) returns the raw `*http.Response` for task 04 to relay, the client's `Timeout` clock continues through the caller's body read and can interrupt it. Fine for small OTLP ingest responses, but worth a one-line comment on the method so the task-04 implementer knows the timeout envelopes the relay too, not just the round-trip to first byte.
- **Streaming an arbitrary `io.Reader` body sends chunked transfer-encoding.** Passing a non-`*bytes.Reader`/`*strings.Reader` body to `http.NewRequestWithContext` leaves `ContentLength` unset, so the client uses `Transfer-Encoding: chunked` to Loki. This is the correct consequence of the no-buffering rule (buffering to compute a length would violate label-blindness), and Loki's OTLP receiver accepts chunked — just noting it's expected, not a defect.
- **`Content-Type` is set unconditionally while `Content-Encoding` is guarded on non-empty.** If `contentType == ""`, the plan sets an empty `Content-Type` header (Go only auto-detects when the header is entirely absent, so it stays empty). OTLP writes always carry a Content-Type, so this is theoretical; for symmetry and faithful pass-through the implementer may consider the same non-empty guard, but it's not required.

### Positive Notes

- Task decomposition (1 → 2 → 3) is clean and each task names the single file it touches (`internal/proxy/forwarder.go`), with correct dependency ordering.
- The label-blind contract is stated forcefully and repeatedly ("never read, buffer, decompress, or parse it"), and the hop-by-hop header list is enumerated in full — matching the spec note's watch-list.
- The design inherently prevents hop-by-hop leakage: `Forward` receives only `contentType`/`contentEncoding` as scalars, not a header map, so there is no `Header` set to accidentally copy from.
- Trailing-slash normalization, ctx propagation, no-retry rationale, and "never swallow Loki's error body" (relay 4xx/5xx verbatim) are all captured, matching the note's edge-case list.
- Constructor resolves the full target once so the hot path builds no URLs — a sensible micro-decision that's explicitly reasoned.
- House-style adherence is called out (package doc as prose describing behavior, sentinel-error pattern mirroring `config.go`).

PLAN_REVIEW_PASS
