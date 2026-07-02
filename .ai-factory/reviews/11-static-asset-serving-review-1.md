# Code Review: Static asset serving (plan 11)

**Scope:** `web_embed.go`, `internal/admin/static.go`, `cmd/proxy/main.go` (+ plan/artifact files, non-code).

## What was reviewed

Read all three changed Go files in full plus their surrounding context (`newMux`, the existing route table, the `admin` package). Verified the build, `go vet`, and — because the milestone's guarantees are behavioral (route shadowing, CWD-independence, no SPA fallback) — exercised the running binary end-to-end.

## Build / vet

- `go build ./...` — OK
- `go vet ./...` — OK

## Runtime verification

Started the binary with `DB_PATH=/tmp/... PROXY_LISTEN=:48318` and hit every route:

| Request | Result | Expected? |
|---|---|---|
| `GET /` | `200`, `text/html`, serves embedded `index.html` | ✅ GUI served from embed |
| `GET /healthz` | `200 ok` | ✅ not shadowed by `/` |
| `GET /v1/logs` | `401 unauthorized` (write-plane auth) | ✅ not answered by file server |
| `GET /api/tokens` | `401 unauthorized` (Grafana gate) | ✅ not answered by file server |
| `GET /nonexistent.js` | `404 page not found` | ✅ plain file-server 404, no SPA index fallback |
| `GET /index.html` | `301` → `/` | ✅ stock `http.FileServer` canonicalization, benign |

The page was served from the embedded FS regardless of the DB/working-directory location, confirming the no-CWD-dependency requirement.

## Correctness notes

- **Route shadowing:** `mux.Handle("/", static)` is safe. `http.ServeMux` uses longest-prefix matching, so the more specific `/healthz`, `/v1/logs`, `/api/tokens`, `/api/tokens/` patterns win over the `/` catch-all irrespective of registration order. Confirmed at runtime above.
- **Embed placement:** `//go:embed web` correctly lives in the module-root `web_embed.go` (`package webassets`); `internal/admin/static.go` imports it as `webassets "github.com/mind-systems/observe-write-proxy"` and roots it with `fs.Sub(webassets.FS, "web")`. This is the only feasible arrangement given Go's embed rules (no `..`, pattern relative to source dir) with `web/` fixed at the module root — matches the plan's Assumptions.
- **Error handling:** `NewStaticHandler`'s `fs.Sub` error is propagated and made fatal in `main` (`Fprintln` + `os.Exit(1)`), consistent with the existing fatal-wiring pattern. In practice this path is unreachable for a binary that compiled with the embed present; the defensive signature is harmless.
- **`handleNotImplemented` removal:** the only remaining `/` binding is the static handler; no other caller referenced the removed function (`go build`/`vet` confirm no dead reference).
- **Plane separation preserved:** new import edges are `internal/admin → webassets` and the pre-existing `cmd/proxy → internal/admin`; `internal/proxy` is untouched and `webassets` imports only `embed`. No cycle, no OTLP-body parsing introduced.

## Findings

None. The change builds, vets, runs, and satisfies every behavioral guarantee in the spec (embedded GUI at `/`, CWD-independent, specific routes not shadowed, no SPA fallback).

REVIEW_PASS
