# Code Review 1: 03 — Loki forwarder

## Scope
- **Changed code:** `internal/proxy/forwarder.go` (new, 119 lines) — the only production file in this diff.
- Other staged files are planning artifacts (`plans/03-loki-forwarder.md`, `plans/03-loki-forwarder.json`, `plan-reviews/03-loki-forwarder-plan-review-1.md`) — reviewed for context, not code.
- Verified: `go build ./...` (OK), `go vet ./internal/proxy/` (OK), `gofmt -l` (clean).

## Verdict
No correctness, security, or robustness **bugs** found. The implementation faithfully realizes the plan and the spec note, honors the label-blind contract and the dependency rules, and is standard-library-only. One deviation from the plan is an improvement, not a defect (see below). A few informational observations are recorded that require **no change** in this task.

## Correctness analysis

### Error classification is correct — and better than the plan
The plan specified `os.IsTimeout` for timeout detection. The implementer instead defined a local `timeouter` interface and used `errors.As` (`StatusForError`, forwarder.go:96-118). This is the right call and a genuine improvement:
- `Forward` wraps the transport error with `fmt.Errorf("proxy: forward to loki: %w", err)` (forwarder.go:81). `os.IsTimeout` does **not** unwrap `%w` chains, so it would fail to see the underlying `*url.Error` and misclassify a real timeout as 502.
- `errors.As(err, &te)` walks the wrapped chain, finds the `*url.Error`, and `te.Timeout()` returns true for both an `http.Client.Timeout` expiry and a dial/response timeout → 504.
- `errors.Is(err, context.DeadlineExceeded)` (forwarder.go:110) catches a caller-ctx deadline → 504.
- Connection-refused / DNS "no such host" / TLS errors report `Timeout() == false` → fall through to 502 (forwarder.go:117). Correct.
- `errors.As` with a pointer to the (unexported) interface type is valid and does not panic; the nil-error guard (forwarder.go:107) prevents a spurious classification.

### Pass-through / label-blindness — honored
`Forward` passes `body` straight into `http.NewRequestWithContext` (forwarder.go:73) and never reads, buffers, decompresses, or parses it. Only `Content-Type` (always) and `Content-Encoding` (guarded on non-empty, forwarder.go:77-79) are set; no header map is copied, so hop-by-hop leakage is structurally impossible — `Forward` receives only two scalar header values, not a `http.Header`. The context is threaded through, so caller cancellation/shutdown aborts the upstream call. No retry. All matches spec.

### Response ownership — correctly documented
`Forward` returns the raw `*http.Response` and documents that the caller owns relaying status/body verbatim and closing `resp.Body` (forwarder.go:59-67). This keeps task-04's route as the single writer to the client, as decided in the spec note.

### Constructor / URL join — correct
`NewForwarder` re-validates scheme+host (defensive, mirroring `config.isValidBaseURL`) and fails fast with a wrapped sentinel `ErrLokiURL` (forwarder.go:47-53). `strings.TrimSuffix(lokiBaseURL, "/") + lokiLogsPath` avoids the double-slash trap for the canon base URL. Target is resolved once; the hot path builds no URLs.

## Informational observations (no change required)

1. **Forwarder timeout (30s) equals the server `WriteTimeout` (30s).** `cmd/proxy/main.go:33` sets `WriteTimeout: 30 * time.Second`, which bounds the entire handler including the response write; `forwardTimeout` is also 30s (forwarder.go:22). With zero margin, once task 04 wires the forwarder into `/v1/logs`, a ~30s-slow Loki could let the server envelope cut the connection before a clean 504 can be written. This is the carry-over flagged in plan-review 02 / plan-review 03. The forwarder is not yet wired, so there is no runtime effect today, and resolving it belongs to the task-04 route (per-handler deadline, or a `forwardTimeout` strictly `< WriteTimeout`). Noting it so it is not lost at wiring time — not a defect in this isolated package.

2. **`Content-Type` is set unconditionally** (forwarder.go:76) while `Content-Encoding` is guarded on non-empty. An empty `contentType` would forward an empty `Content-Type` header rather than omitting it. OTLP writes always carry a Content-Type, so this is theoretical; faithful pass-through, and matches the plan verbatim.

3. **Trailing-slash normalization trims a single `/`** and operates on the raw string, not the parsed `*url.URL`. Adequate for the canon base URL (`http://localhost:3100`, no trailing slash) and for a config-validated value; an exotic base with a query/fragment or a double trailing slash could produce an odd target, but config governs this input and the canon default is clean.

REVIEW_PASS
