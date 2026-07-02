# Plan: /v1/logs route

## Context
Add the `POST /v1/logs` HTTP handler that fronts the write plane: guard the method, read the content headers, hand the streamed body to the existing `Forwarder`, and relay Loki's status and body verbatim — no auth yet (task 07 slots that in as middleware).

## Settings
- Testing: no
- Logging: minimal
- Docs: no

## Tasks

### Phase 1: Handler

- [x] **Task 1: Write the `/v1/logs` handler in `internal/proxy`**
  Files: `internal/proxy/handler.go`
  Add a handler that owns a `*Forwarder` (the only dependency) and satisfies `http.Handler`. Define a `Handler` struct `{ fwd *Forwarder }` with a constructor `NewHandler(fwd *Forwarder) *Handler` and a `ServeHTTP(w http.ResponseWriter, r *http.Request)` method (so task 07 can wrap it as middleware without editing it). Logic, in order:
  - **Method guard:** if `r.Method != http.MethodPost`, set `w.Header().Set("Allow", "POST")` then `http.Error(w, ..., http.StatusMethodNotAllowed)` and return — do this before touching the forwarder, so GET never forwards.
  - **Read content headers:** `contentType := r.Header.Get("Content-Type")` and `contentEncoding := r.Header.Get("Content-Encoding")`. Do not require, validate, or default them — the proxy is content-agnostic.
  - **Forward:** call `resp, err := h.fwd.Forward(r.Context(), r.Body, contentType, contentEncoding)`, passing `r.Body` directly (never read/buffer it — streaming + label-blind guarantee).
  - **On error:** write `proxy.StatusForError(err)` (the forwarder's 502/504 classifier) with a short plain-text message via `http.Error`; never return 200 for a failed forward.
  - **On success:** `defer resp.Body.Close()`; copy the upstream `Content-Type` onto `w.Header()` first, then `w.WriteHeader(resp.StatusCode)`, then `io.Copy(w, resp.Body)`. Order matters — any header set after `WriteHeader` is dropped. Loki's 2xx and its 4xx/5xx are both relayed unchanged (do not invent a status).
  Follow the existing `internal/proxy/forwarder.go` doc-comment style. Do not inspect, parse, or mutate the OTLP body anywhere.

- [x] **Task 2: Wire the handler into `cmd/proxy`** (depends on Task 1)
  Files: `cmd/proxy/main.go`
  In `main`, after loading config, construct the forwarder: `fwd, err := proxy.NewForwarder(cfg.LokiURL)` and fail fast (print to stderr, `os.Exit(1)`) on error, matching the existing config-load error handling. Pass `fwd` into `newMux` (change its signature to `newMux(fwd *proxy.Handler)` or `newMux(h *proxy.Handler)` — build the `*proxy.Handler` in `main` via `proxy.NewHandler(fwd)` and pass it in). Replace the `mux.HandleFunc("/v1/logs", handleNotImplemented)` placeholder with `mux.Handle("/v1/logs", h)`. Leave `/api/tokens`, `/api/tokens/`, and `/` on `handleNotImplemented` (later tasks). Add the `internal/proxy` import.

## Notes
- Auth is intentionally absent: with no Bearer check wired, any caller is accepted — that gap is closed by task 07, which wraps this handler as middleware without editing it.
- Do not add batching, decompression, or any OTLP inspection/rewrite.
