# Plan: Static asset serving

## Context
The single static proxy binary carries the admin GUI inside itself and serves it at `/`, so an operator opens the management page with no external files and no dependency on the working directory the binary runs from.

## Settings
- Testing: no
- Logging: minimal
- Docs: no

## Assumptions
- **Embed must live at the module root.** `//go:embed` interprets patterns relative to the source file's directory and forbids `..`. `web/` is fixed at the module root by `ARCHITECTURE.md`, so the `//go:embed web` directive (and the raw `embed.FS`) has to be in a Go file at the module root — it cannot be written inside `internal/admin`. To honor the spec's intent that `internal/admin` owns serving, the split is: the module-root file only holds the embedded `embed.FS`; the `fs.Sub` + `http.FileServer` construction lives in `internal/admin`.
- Route shadowing is already prevented by `http.ServeMux` longest-prefix matching: `/healthz`, `/v1/logs`, `/api/tokens`, `/api/tokens/` are more specific patterns than `/` and win regardless of registration order. The existing `/` placeholder (`handleNotImplemented`) is simply replaced.

## Tasks

### Phase 1: Embed and serve the GUI

- [x] **Task 1: Embed `web/` into the binary at the module root**
  Files: `web_embed.go`
  Create a new Go file at the module root declaring `package webassets` with `import "embed"` and:
  ```go
  //go:embed web
  var FS embed.FS
  ```
  This is the only place the `//go:embed` directive can reach the root `web/` directory. Keep the file to just the embed declaration and a short doc comment explaining why the embed lives at the root (Go's no-`..` embed rule) rather than in `internal/admin`. `web/index.html` already exists as a placeholder, so the directive compiles immediately.

- [x] **Task 2: Add the static file-server handler in `internal/admin`** (depends on Task 1)
  Files: `internal/admin/static.go`
  Add `NewStaticHandler() (http.Handler, error)` in `package admin`. Root the embedded FS at `web/` with `sub, err := fs.Sub(webassets.FS, "web")` (import the module-root package `github.com/mind-systems/observe-write-proxy` as `webassets`); on error return it (mis-built binary — fatal wiring, not a runtime 404). On success return `http.FileServer(http.FS(sub))`. This strips the `web/` prefix so assets serve at `/`, `/app.js`, `/style.css`. Add a doc comment noting the page is public (it only collects the operator's Grafana token client-side) and that a miss is a plain `404` from the file server — no SPA index fallback. Follow the existing `internal/admin` doc-comment style.

- [x] **Task 3: Wire the static handler onto `/` in the mux** (depends on Task 2)
  Files: `cmd/proxy/main.go`
  In `main`, after constructing the other handlers, call `admin.NewStaticHandler()`; on error print to stderr and `os.Exit(1)` (consistent with the existing fatal-wiring pattern). Pass the resulting `http.Handler` into `newMux` and register it with `mux.Handle("/", static)`, replacing the `mux.HandleFunc("/", handleNotImplemented)` placeholder. Remove the now-unused `handleNotImplemented` function. Leave `/healthz`, `/v1/logs`, `/api/tokens`, `/api/tokens/` registrations unchanged — ServeMux longest-prefix matching keeps them ahead of the `/` catch-all. Update `newMux`'s signature/doc comment to take the static handler instead of describing `/` as a placeholder.
