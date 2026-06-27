# 04 — /v1/logs route

**Task:** ROADMAP → Phase 2 — Write-path forwarding → "/v1/logs route"
**Depth:** low

## Goal

The `POST /v1/logs` HTTP handler that fronts the write plane: it guards the method, reads the content headers off the request, hands the streamed body to the `Forwarder`, and writes Loki's status code and body straight back to the caller — with no authentication yet, so any caller passes for now.

## Design

- Handler lives in `internal/proxy`, constructed with a `*Forwarder` (from task 03) so it has no other dependencies. Registered on the standard `http.ServeMux` at path `/v1/logs` and wired in `cmd/proxy`.
- **Method guard:** Go 1.19's `http.ServeMux` has no method patterns, so the handler inspects `r.Method` itself. Non-`POST` → `405 Method Not Allowed`; set the `Allow: POST` header on the 405 response per HTTP semantics.
- **Read content headers:** pull `Content-Type` and `Content-Encoding` from `r.Header`; pass them through to `Forwarder.Forward` unchanged. Do not require, validate, or default them — the proxy is label-blind and content-agnostic.
- **Forward:** call `Forward(r.Context(), r.Body, contentType, contentEncoding)`, passing `r.Body` directly as the stream so nothing is buffered in the handler. The request context carries client-disconnect/shutdown cancellation into the upstream call.
- **Relay the response:**
  - On a non-nil error from `Forward` (upstream unreachable/timeout, already classified by the forwarder), write the mapped 5xx (`502`/`504`) with a short plain message; do not 200 a failed forward.
  - On success, copy the upstream `Content-Type` to the response, `WriteHeader` with Loki's exact status code, then `io.Copy` Loki's body to the `ResponseWriter` so 2xx and Loki 4xx/5xx are both relayed verbatim. Close the upstream response body (`defer resp.Body.Close()`).
- The handler is intentionally a thin shell so that Phase 4 auth (task 07) can be slotted in **as middleware wrapping this handler** without editing it.

## Edge cases / watch

- **Do not read `r.Body` before forwarding** — buffering would break streaming and the label-blind guarantee, and could OOM on large batches. Pass it through.
- **405 needs the `Allow` header** so a misbehaving client learns the right method; an empty 405 is a smell.
- **Write headers in the right order** — set `Content-Type`, then `WriteHeader(status)`, then copy the body; any header set after `WriteHeader` is silently dropped.
- **Do not invent a status** — a reachable Loki returning 4xx must surface as that same 4xx, not be masked as a proxy error; only a transport-level failure becomes a proxy 5xx (the forwarder draws that line).
- **GET to `/v1/logs` must not forward** — the method guard returns 405 before the forwarder is ever touched.

## Out of scope

- Bearer authentication / token-store checks — Phase 4 (task 07), added as middleware in front of this handler, leaving it unchanged.
- Batching, decompression, or any inspection/rewrite of the OTLP body.
- The forwarder's construction and failure-classification logic (task 03).

## Done when

- `POST /v1/logs` with an OTLP body forwards to Loki and returns Loki's exact status code and body.
- A non-`POST` request to `/v1/logs` returns `405` with an `Allow: POST` header and never calls the forwarder.
- With no auth wired yet, any caller (no/any token) is accepted — that gap is left open for task 07.
