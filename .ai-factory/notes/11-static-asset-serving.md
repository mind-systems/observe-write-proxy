# 11 — Static asset serving

**Task:** ROADMAP → Phase 6 — Admin GUI → "Static asset serving"
**Depth:** low

## Goal

The single static proxy binary carries the admin GUI inside itself and serves it at `/`, so an operator can open the management page with no external files and no dependency on the working directory the binary runs from.

## Design

- Embed the `web/` directory into the binary at compile time with a `//go:embed web` directive backed by an `embed.FS`. The embed lives next to the package that owns admin serving (`internal/admin`), so the GUI assets travel with the admin plane that uses them.
- Root the embedded filesystem at `web/` using `fs.Sub(embedded, "web")`, then serve it with `http.FileServer(http.FS(sub))`. This exposes `web/index.html`, `web/app.js`, and any `web/style.css` directly under `/` (e.g. `/`, `/app.js`, `/style.css`) without the `web/` prefix leaking into URLs.
- `internal/admin` exposes the file server as the handler for `/`; `cmd/proxy` mounts it on the shared `http.ServeMux` alongside the other routes during wiring. The proxy/write plane is untouched — this is admin-plane only.
- The static page is **public**: it is served with no auth. It only collects the operator's Grafana token client-side and never holds proxy secrets. The gated surface is the `/api/tokens` API (Grafana-token Bearer per request), not the page that calls it. Serving the HTML to an unauthenticated browser leaks nothing.

## Edge cases / watch

- With `http.ServeMux`, the `/` pattern is a catch-all prefix and will shadow every unregistered path. The specific routes (`/v1/logs`, `/api/tokens`, `/healthz`) must be registered as their own patterns so the mux's longest-prefix match routes them to their handlers before falling back to the `/` file server. Verify a request to `/v1/logs` and `/api/tokens` is never answered by the file server.
- `fs.Sub` returns an error if `web/` is missing from the embed; treat a failed `fs.Sub` at startup as fatal wiring (the binary is mis-built), not a runtime 404.
- The GUI is a single page with no client-side router, so no SPA deep-link fallback / rewrite-to-index is needed. A miss under `/` should be a plain `404` from the file server, not a redirect to `index.html`.
- Keep `http.FileServer`'s default directory listing in mind: only the intended `web/` assets are embedded, so a listing exposes nothing sensitive, but the page set should stay limited to the GUI files.

## Out of scope

- The content of the GUI (`index.html`, `app.js`, styling) and its behavior — covered by note 12.
- Auth on `/api/tokens` and the Grafana validation path — owned by the admin/grafana packages, not this serving task.
- Any caching headers, compression, or asset fingerprinting — not required for a single local page.

## Done when

Hitting `/` on the running proxy serves the embedded GUI page and its assets (`app.js`, any `style.css`) straight from the binary, the binary serves them identically regardless of the current working directory, and `/v1/logs`, `/api/tokens`, and `/healthz` continue to reach their own handlers rather than the file server.
