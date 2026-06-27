# 03 — Loki forwarder

**Task:** ROADMAP → Phase 2 — Write-path forwarding → "Loki forwarder"
**Depth:** medium

## Goal

A label-blind forwarder that streams an incoming OTLP/HTTP log body to Loki's OTLP ingest endpoint byte-for-byte, preserving the content headers, and turns upstream transport failures into sane proxy 5xx responses while relaying Loki's own status and body on success or rejection.

## Design

- New package `internal/proxy`. It holds the write plane and imports `store` only (never `admin`/`grafana`); this task introduces no `store` use yet.
- A `Forwarder` type built once at startup, holding:
  - the resolved target URL — the configured Loki base URL with `/otlp/v1/logs` appended (default base `http://localhost:3100`, so default target `http://localhost:3100/otlp/v1/logs`); resolve and store the full string at construction so the hot path does no URL building.
  - an `*http.Client` with an explicit request timeout (sane default, e.g. 30s) so a hung Loki cannot pin a goroutine forever.
- Constructor `NewForwarder(lokiBaseURL string) (*Forwarder, error)` (or accepting an already-resolved client) — validate/normalise the base URL once (trim trailing slash before appending the path) and fail fast on a malformed URL.
- Forward method, e.g. `Forward(ctx context.Context, body io.Reader, contentType, contentEncoding string) (*http.Response, error)`:
  - Build a `POST` to the target with `http.NewRequestWithContext(ctx, …)` so caller cancellation/deadline propagates to the upstream call.
  - Pass `body` straight into the request as the `io.Reader` — **stream it through unchanged**. Do not read, buffer, decompress, or parse it. The proxy is label-blind: it never touches OTLP attributes or Loki labels.
  - Copy `Content-Type` from the incoming request onto the upstream request. Copy `Content-Encoding` only when non-empty so an absent encoding is not forced to a literal empty header.
  - Return the raw `*http.Response` to the handler; the handler owns relaying status + body and closing the body. (If this method instead writes the response itself, it must still stream the upstream body back and propagate the status verbatim — pick one and keep the handler thin; **Decision:** return the `*http.Response` so the route in task 04 stays the single place that writes to the client.)
- Failure mapping is the forwarder's job to classify, the handler's to render:
  - A non-nil error from `client.Do` (connection refused, DNS failure, TLS error, dial/response timeout) means the request never got a verdict from Loki → the proxy must surface a `502` (bad proxy) or, when the failure is a deadline/timeout, `504` (proxy timeout). Distinguish timeout via `errors.Is(err, context.DeadlineExceeded)` / `os.IsTimeout` to choose 504 vs 502.
  - A nil error means Loki answered: relay its status code and body unchanged, including 4xx/5xx — the proxy adds no opinion of its own.

## Edge cases / watch

- **Never swallow Loki's error body.** When Loki returns 4xx/5xx the body carries the reason; it must reach the caller, not be discarded in favour of a generic message.
- **Respect ctx cancellation** — the request must be built with the caller's context so a client disconnect or server shutdown aborts the in-flight upstream call.
- **Do not copy hop-by-hop headers** (`Connection`, `Keep-Alive`, `Proxy-*`, `TE`, `Trailer`, `Transfer-Encoding`, `Upgrade`). Only `Content-Type` and `Content-Encoding` are propagated; let the `http.Client` manage transport framing.
- **Always close the upstream response body** to avoid leaking connections — whoever consumes the `*http.Response` (the route) owns this; note it so it is not forgotten.
- **Trailing-slash trap:** appending `/otlp/v1/logs` to a base that already ends in `/` yields a double slash; normalise the base once at construction.
- **No retries** — a retry would risk duplicate ingestion of a non-idempotent log batch; surface the failure instead.

## Out of scope

- The HTTP route, method guard, and reading headers off the wire (task 04 — `/v1/logs` route).
- Bearer authentication against the token store (Phase 4 / task 07).
- Any inspection, batching, decompression, or rewriting of the OTLP payload — permanently out of scope by canon.

## Done when

- A `POST` carrying an OTLP/HTTP body reaches Loki's `…/otlp/v1/logs` with the body bytes and `Content-Type`/`Content-Encoding` intact (verifiable by Loki accepting the write).
- A successful or Loki-rejected request relays Loki's exact status code and body back to the caller.
- An unreachable or slow Loki (connection refused / DNS failure / timeout) surfaces as a proxy `502` (or `504` on timeout), not a panic or a hang, with the upstream error not silently swallowed.
