# Plan Review: 13 — Build automation

## Code Review Summary

**Files Reviewed:** plan `13-build-automation.md`, spec note `13-build-automation.md`, plus codebase (`web_embed.go`, `cmd/proxy/main.go`, `go.mod`, `.gitignore`, `ARCHITECTURE.md`)
**Risk Level:** 🟢 Low

### Context Gates
- **Architecture** (`.ai-factory/ARCHITECTURE.md`): WARN — none. The plan aligns with the declared layout: `cmd/proxy/` is the main entry (build target `./cmd/proxy`), `web/` lives at the module root and is served by the admin plane. No dependency-rule or boundary implications — a Makefile is build tooling, it touches no package graph.
- **Rules** (`.ai-factory/rules/base.md` present; no `RULES.md`): WARN — none observed. "Standard-library-first / no extra tooling" canon is respected — the plan explicitly forbids Taskfile/mage and stays on plain `go`.
- **Roadmap** (`.ai-factory/ROADMAP.md`): WARN — none. The spec note anchors this to *Phase 7 — Packaging & native run → "Build automation"*, so milestone linkage is present.
- **skill-context** (`aif-review/SKILL.md`): not present in this repo or the root — no project-specific review overrides to apply.

### Critical Issues
None.

### Verification of Plan Assumptions (all confirmed against the codebase)
- **`./cmd/proxy` build path** — `cmd/proxy/main.go` exists with `package main`. Correct target. ✅
- **`//go:embed` for `web/`** — the plan (Task 1) correctly points to `web_embed.go` at the module root (`//go:embed web`, package `webassets`), which `internal/admin` roots via `fs.Sub`. Note: the **spec note** (line 16) is stale here — it claims the embed lives "in `internal/admin`". The plan supersedes the note with the accurate location, so this is not a defect in the plan; flagging only so the note isn't trusted over the plan.
- **`CGO_ENABLED=0` + `modernc.org/sqlite`** — `go.mod` requires `modernc.org/sqlite v1.28.0` (pure-Go driver), so the no-cgo build is valid and the invariant holds. ✅
- **`/bin/` already gitignored** — confirmed at `.gitignore:8` (`/bin/`). Task 2 correctly states no edit is needed. Here again the **spec note** (line 24) is stale — it claims `/bin/` is *missing* and must be added. The plan caught this and downgraded Task 2 to a verify-only step. Good catch; the plan is more accurate than its own source note.
- **Toolchain compatibility** — `go 1.19`; all recipe commands (`go build`, `go run`, `go test ./...`, `gofmt`, `go vet ./...`, `go mod tidy`) are valid on that version. ✅
- **`-ldflags "-s -w"` quoting** — the whole `go build` is a single recipe line, so the inline `CGO_ENABLED=0` scopes to that one command and the quoted ldflags survive `/bin/sh` intact. The plan explicitly calls out the single-line requirement. ✅

### Minor Notes (non-blocking)
- `.gitignore` also carries a stale `/observe-write-proxy` entry (line 9, an old root-binary name) alongside `/bin/`. Out of scope for this task and harmless — noting only for awareness; the plan is right to leave it.
- Scope boundaries are clean: Dockerfile/compose (task 15), README run docs (task 14), and E2E write verification (task 16) are correctly deferred, so this plan stays a pure Makefile addition with no hidden coupling.

### Positive Notes
- Every load-bearing decision (`CGO_ENABLED=0` inline vs exported, `run` via `go run` not depending on `build`, vet folded into `fmt`, no `-trimpath` unprompted) is justified in both the plan and note — an implementer has no ambiguity to guess through.
- The plan actively corrects two stale claims in its own spec note (embed location, `/bin/` gitignore state) rather than propagating them — a sign the codebase was actually read, not just the note.
- Correctly identifies that `bin/` is auto-created by `go build -o bin/proxy`, so no `mkdir` step is needed.

PLAN_REVIEW_PASS
