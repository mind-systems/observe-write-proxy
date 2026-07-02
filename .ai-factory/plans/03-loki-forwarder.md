# Plan: Loki forwarder

## Context
Introduce the write plane's core: a label-blind `internal/proxy.Forwarder` that streams an incoming OTLP/HTTP log body to Loki's `/otlp/v1/logs` endpoint byte-for-byte, relays Loki's verdict, and maps transport failures to `502`/`504`.

## Settings
- Testing: no
- Logging: minimal
- Docs: no

## Tasks

### Phase 1: Forwarder

- [x] **Task 1: Create the `internal/proxy` package and `Forwarder` type**
  Files: `internal/proxy/forwarder.go`
  Create the new package `proxy` (package doc comment matching the house style used in `internal/config/config.go` — prose describing behavior, why it is label-blind). Define an exported `Forwarder` struct holding:
  - `target string` — the fully resolved forward URL (`<loki-base>/otlp/v1/logs`), computed once at construction so the hot path builds no URLs.
  - `client *http.Client` — an `http.Client` with an explicit `Timeout` (default `30 * time.Second`, declared as a named `const`) so a hung Loki cannot pin a goroutine indefinitely.
  Keep imports to the standard library only; this package must not import `internal/admin` or `internal/grafana` (dependency rule from `.ai-factory/ARCHITECTURE.md`). No `store` use in this task.

- [x] **Task 2: `NewForwarder` constructor with base-URL normalisation** (depends on Task 1)
  Files: `internal/proxy/forwarder.go`
  Add `NewForwarder(lokiBaseURL string) (*Forwarder, error)`:
  - Validate the base URL with `url.Parse`, failing fast (wrapped, named error) on a malformed or scheme/host-less value — mirror the `isValidBaseURL` intent in `internal/config/config.go` (config already validates, but the forwarder must not trust silently).
  - Normalise the base once: trim any trailing `/` before appending `/otlp/v1/logs`, avoiding the double-slash trap noted in the spec. Store the result in `target`.
  - Construct the `http.Client` with the timeout const and return the assembled `*Forwarder`.
  Define a package sentinel error (e.g. `ErrLokiURL`) following the `errors.New` + `fmt.Errorf("proxy: …: %w", …)` pattern used in config.

- [x] **Task 3: `Forward` method — stream, relay, and classify failures** (depends on Task 2)
  Files: `internal/proxy/forwarder.go`
  Add `Forward(ctx context.Context, body io.Reader, contentType, contentEncoding string) (*http.Response, error)`:
  - Build the request with `http.NewRequestWithContext(ctx, http.MethodPost, f.target, body)` so caller cancellation/deadline propagates upstream. Pass `body` straight through — **never read, buffer, decompress, or parse it**.
  - Set `Content-Type` from `contentType`. Set `Content-Encoding` only when `contentEncoding != ""` so an absent encoding is not forced to an empty header. Copy no other headers — no hop-by-hop headers (`Connection`, `Keep-Alive`, `Proxy-*`, `TE`, `Trailer`, `Transfer-Encoding`, `Upgrade`); let the client manage transport framing.
  - Call `f.client.Do(req)`. On a nil error, return the raw `*http.Response` and `nil` — the caller (task 04 route) owns relaying status + body verbatim (including 4xx/5xx) and closing the body. Document in a comment that the caller must close `resp.Body`.
  - On a non-nil error, return `nil` plus a classified error so the handler can render `502` vs `504`. Provide the classification: expose a helper `StatusForError(err error) int` (or return a typed error carrying the intended status) that maps timeout/deadline failures (`errors.Is(err, context.DeadlineExceeded)` / `os.IsTimeout`) to `http.StatusGatewayTimeout` (504) and all other transport failures (connection refused, DNS, TLS) to `http.StatusBadGateway` (502). Do not retry (avoids duplicate ingestion) and do not swallow the underlying error — wrap it so the reason survives.

## Notes for implementer
- No tests, no report, no route wiring — the `POST /v1/logs` handler, method guard, and reading headers off the wire are task 04 (out of scope here).
- Single logical change → one commit at the end: "Add label-blind Loki forwarder to internal/proxy".
