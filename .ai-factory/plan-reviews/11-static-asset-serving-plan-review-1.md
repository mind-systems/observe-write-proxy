# Plan Review: Static asset serving (plan 11)

**Plan:** `.ai-factory/plans/11-static-asset-serving.md`
**Files Reviewed:** plan + `cmd/proxy/main.go`, `internal/admin/{auth,tokens}.go`, `web/index.html`, `go.mod`, `ARCHITECTURE.md`, ROADMAP Phase 6, spec note 11
**Risk Level:** 🟢 Low

## Context Gates

- **Architecture (`ARCHITECTURE.md`)** — WARN (informational). The folder-structure block lists `cmd/`, `internal/`, `web/` and no root-level Go package. Task 1 adds `web_embed.go` (`package webassets`) at the module root. This is a benign deviation and the plan justifies it correctly (see below); the doc's own note ("Concrete package layout is a starting point — refine as the service takes shape") sanctions it. With `Docs: no`, ARCHITECTURE.md won't be updated — acceptable for a low-depth task, but worth a one-line refresh later.
- **Dependency rules** — PASS. New import edges are `internal/admin → webassets(root)` and `cmd/proxy → internal/admin` (already exists). Root `webassets` imports only `embed`. No cycle; write plane (`internal/proxy`) untouched; no package parses the OTLP body. The two-plane spine is preserved.
- **Roadmap** — PASS. Maps to Phase 6 → "Static asset serving"; spec note `11-static-asset-serving.md` present and referenced.

## Critical Issues

None.

## Notable Strengths

- **The plan correctly overrides an infeasible spec-note design.** Spec note 11 states "the embed lives next to the package that owns admin serving (`internal/admin`)". That is not possible: `//go:embed` resolves patterns relative to the source file's directory and forbids `..`, so a file in `internal/admin` cannot reach the module-root `web/`. The plan's Assumptions section catches this precisely and adapts — embed `embed.FS` at the module root, keep the `fs.Sub` + `http.FileServer` construction in `internal/admin` — honoring the spec's *intent* (admin owns serving) while respecting Go's embed constraint. This is the correct resolution.
- **Route-shadowing assumption is accurate.** Go's `http.ServeMux` uses longest-prefix matching; `/healthz`, `/v1/logs`, `/api/tokens`, `/api/tokens/` are more specific than `/` and win regardless of registration order. Verified against the current `newMux`. Replacing only the `handleNotImplemented` binding on `/` is safe.
- **Toolchain is sufficient.** `go.mod` declares `go 1.19` and the installed toolchain is `go1.19.3`; `embed`, `io/fs.Sub`, and `http.FS` are all available since 1.16. No cgo introduced — the single-static-binary / `CGO_ENABLED=0` invariant holds.
- **`web/index.html` exists**, so `//go:embed web` compiles immediately (an empty/all-excluded directory would fail with "no matching files found"). The plan relies on this correctly.
- **Fatal-wiring error handling is consistent** with the existing `main.go` pattern (`fmt.Fprintln(os.Stderr, err); os.Exit(1)`), and the public/ungated `/` matches the design already documented in `newMux`'s doc comment and note 09.

## Minor Notes (non-blocking)

- **`NewStaticHandler` error path is effectively unreachable.** `fs.Sub(webassets.FS, "web")` can only fail on an invalid path or a non-`SubFS` — neither can happen for a binary that compiled with the embed present. The `(http.Handler, error)` signature is defensive rather than load-bearing; it's fine and matches the codebase's fatal-wiring style, just don't expect it to fire.
- **`http.FileServer` directory listing.** For any embedded directory lacking an `index.html`, `http.FileServer` renders a listing. Currently `web/` holds only `index.html`, so `/` serves the page and nothing sensitive is exposed; note 11 already flags this and Task 2's "no SPA fallback, plain 404" instruction is correct. Just keep the embedded set limited to GUI files (relevant when Task 12 adds `app.js`/`style.css`).
- **Root package naming.** `package webassets` at the module root means any future root-level `.go` file must also be `package webassets`. Harmless, but a mild surprise for readers expecting the root to be package-less. A short doc comment (the plan already asks for one) mitigates this.

## Verdict

The plan is well-scoped, correctly sequenced (embed → handler → wiring, with explicit dependencies), and technically accurate down to the Go embed constraint and ServeMux semantics. It improves on its own spec note rather than blindly following it. No blocking issues.

PLAN_REVIEW_PASS
