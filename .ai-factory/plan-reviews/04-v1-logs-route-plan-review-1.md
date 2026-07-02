# Plan Review: /v1/logs route (04)

## Code Review Summary

**Files Reviewed:** plan `04-v1-logs-route.md` against `internal/proxy/forwarder.go`, `internal/config/config.go`, `cmd/proxy/main.go`, `go.mod`, ARCHITECTURE.md, rules, ROADMAP.md
**Risk Level:** 🟢 Low

The plan is technically accurate. Every API reference, signature, field name, and import path it cites matches the actual codebase.

### Verification of factual claims

- **`proxy.NewForwarder(cfg.LokiURL)`** — `Config.LokiURL` exists (config.go:21) and `NewForwarder(lokiBaseURL string) (*Forwarder, error)` exists (forwarder.go:46). ✓
- **`h.fwd.Forward(r.Context(), r.Body, contentType, contentEncoding)`** — matches `Forward(ctx context.Context, body io.Reader, contentType, contentEncoding string) (*http.Response, error)` (forwarder.go:76). ✓
- **`proxy.StatusForError(err)`** — exists and returns an `int` (forwarder.go:107). ✓
- **Import path `github.com/mind-systems/observe-write-proxy/internal/proxy`** — module path confirmed in go.mod; package `proxy` already exists. ✓
- **`newMux()` currently takes no args and mounts `/v1/logs` on `handleNotImplemented`** (main.go:26, 72, 76) — the plan's signature change and `mux.Handle("/v1/logs", h)` replacement are correct. ✓
- **Forwarder returns `nil` response on transport error** (forwarder.go:88–90) — so the plan's "defer close only on success" is correct; there is no body to close on the error path. ✓
- **405 non-POST guard** — aligns with ROADMAP ("405 on non-POST"). ✓
- **Header-before-WriteHeader ordering note** — correct; headers set after `WriteHeader` are dropped by `net/http`. ✓

### Context Gates

- **Architecture (ARCHITECTURE.md):** PASS. Dependency rule `proxy/ → store/ + config` is respected — the handler depends only on `*Forwarder`, imports neither `admin/` nor `grafana/`, and does not parse/mutate the OTLP body. The path-split-planes and label-blind invariants are preserved.
- **Rules (rules/base-rules.md):** PASS with one WARN (see below). Naming (`Handler`, `NewHandler`, `ServeHTTP`), `snake_case.go` filename (`handler.go`), and the "surface upstream Loki failure as 5xx, don't swallow" requirement are all honored via `StatusForError`.
- **Roadmap (ROADMAP.md):** PASS. Task maps directly to the `[ ] /v1/logs route` milestone, and the "auth deferred to task 07 middleware without touching this handler" framing matches the `Bearer auth on /v1/logs` milestone in Phase 4. Note the ROADMAP references spec `.ai-factory/notes/04-write-logs-route.md`; the plan does not cite it — not blocking, but confirm the spec note and plan agree.

### Non-blocking Notes (WARN)

1. **No operational logging on forward failure (rules/base-rules.md → Error Handling: "upstream Loki failure … logged").**
   The plan's "Logging: minimal" setting means a failed forward returns 502/504 to the client but records nothing server-side, leaving the operator no signal to diagnose a down/refusing Loki. The base rules say upstream failures should be *logged* (not just surfaced). Consider one line logging the classified status + wrapped error (never the body, never a token) on the error branch. This is a deliberate-tradeoff call, not a defect.

2. **Response `Content-Encoding` is intentionally not relayed — confirm this is fine.**
   The plan copies only the upstream `Content-Type`. This is correct in practice: `Forward` sets no `Accept-Encoding`, so Go's transport transparently requests+decompresses gzip and strips `Content-Encoding` from `resp` — there is normally nothing to relay. Worth stating explicitly in the handler doc-comment so a future reader doesn't "fix" it by blindly copying all headers (which would reintroduce a hop-by-hop leak the forwarder deliberately avoids).

3. **`io.Copy(w, resp.Body)` error is unhandled.**
   Standard and acceptable — once `WriteHeader(resp.StatusCode)` is sent the status can't change, so a mid-body copy failure can only be logged, not corrected. Fine to ignore given the minimal-logging setting; just be aware the plan's silent `io.Copy` is by design, not an oversight to flag later.

### Positive Notes

- Handler-owns-`*Forwarder` with a plain `ServeHTTP` is exactly the right shape for task 07 to wrap as middleware without editing this file — the stated design goal is met cleanly.
- Correct insistence on passing `r.Body` through unbuffered preserves the streaming + label-blind guarantee end-to-end.
- Explicit ordering guidance (Content-Type → WriteHeader → io.Copy) preempts the most common `net/http` mistake.
- Method guard placed *before* touching the forwarder correctly prevents a GET from ever reaching Loki.
- Fail-fast on `NewForwarder` error in `main`, matching existing config-load error handling, keeps startup consistent.

PLAN_REVIEW_PASS
