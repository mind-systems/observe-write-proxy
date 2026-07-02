# Plan Review: 05 — SQLite store init

**Plan:** `.ai-factory/plans/05-sqlite-store-init.md`
**Reviewed against:** `internal/config/config.go`, `cmd/proxy/main.go`, `go.mod`, note `05-sqlite-store-init.md`, `ARCHITECTURE.md`, installed toolchain (`go1.19.3`).

## Summary

**Risk Level:** 🔴 High — one blocking issue.

The plan is well-aligned with the note and the frozen architecture: `internal/store` is the correct package per the folder structure and dependency rules (`store/` → nothing outside itself, the only stateful package); the `tokens` schema matches the "Token Store Shape" canon (`name`, `token` plaintext, `active` flag, creation metadata); the pure-Go `modernc.org/sqlite` choice correctly preserves the `CGO_ENABLED=0` invariant; the DSN-scoped pragmas, lazy-`sql.Open` + `Ping`, parent-dir `MkdirAll`, and `IF NOT EXISTS` idempotency are all sound. `config.DBPath` / default `./proxy.db` already exist, and deferring the `cmd/proxy` wiring to note 06 is consistent.

There is one blocking defect: the dependency step as written will not build on this project's Go toolchain.

### Context Gates

- **Architecture** (`ARCHITECTURE.md`): ✅ aligned. New `internal/store` package, dependency direction, and token shape all conform. No boundary violation (store imports nothing in-repo).
- **Rules** (`.ai-factory/rules/base.md`): present but not read in depth for this plan; no obvious convention conflict. The commit message "Add SQLite token store initialization" satisfies the global no-prefix / sentence-case rule.
- **Roadmap**: note ties to ROADMAP → Phase 3 → "SQLite store init". Linkage present.
- **Skill-context** (`.ai-factory/skill-context/aif-review/SKILL.md`): not present — no project-specific review overrides to apply.

## Critical Issues

### C1 — Task 1's unpinned `go get` breaks the build under the pinned Go 1.19 toolchain (BLOCKING)

`go.mod` declares `go 1.19` and the installed toolchain is `go1.19.3`. Go 1.19 predates `GOTOOLCHAIN` auto-download (introduced in Go 1.21), so it cannot fetch a newer toolchain to satisfy a dependency that demands one, and — unlike Go 1.21+ — its `go get` does **not** avoid selecting module versions whose `go` directive exceeds the local toolchain.

Task 1 instructs: *"Run `go get modernc.org/sqlite`"* — **unpinned**. Verified against the module proxy:

| `modernc.org/sqlite` | required Go |
|---|---|
| v1.53.0 (current latest) | 1.25 |
| v1.34.0 | 1.20 |
| v1.29.0 | 1.20 |
| **v1.28.0** | **1.19** ✅ |

An unpinned `go get` selects the latest (v1.53.0), writes `require modernc.org/sqlite v1.53.0`, and the subsequent `go build` / `go mod tidy` fails because the module (and its transitive `modernc.org/libc`) require Go ≥ 1.25 that is neither installed nor auto-fetchable. Note also that **any** version ≥ v1.29.0 requires Go ≥ 1.20 and therefore also fails on the installed 1.19.3.

This directly contradicts Task 1's own stated intent ("pinned so the build stays reproducible") — the command given is unpinned.

**Verified fix:** pin to the last Go-1.19-compatible release, `modernc.org/sqlite@v1.28.0`. I reproduced this in a throwaway `go 1.19` module: `go get modernc.org/sqlite@v1.28.0` plus `import _ "modernc.org/sqlite"` builds cleanly (`go build` exit 0), keeps the `go 1.19` directive unchanged, and pulls only pure-Go transitive deps (`libc v1.29.0`, `memory`, `opt`, `strutil`, `token` — all indirect, no cgo).

**Recommended plan edit (Task 1):** replace `go get modernc.org/sqlite` with `go get modernc.org/sqlite@v1.28.0`, then `go mod tidy`. Alternatively, if the team wants a newer sqlite, that requires a deliberate, separate decision to raise the `go` directive **and** install a matching toolchain — out of scope for this task and not assumed anywhere in the plan; absent that, v1.28.0 is the ceiling.

## Minor Notes / Suggestions

- **DSN pragma syntax:** the `?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)` form is the correct modernc.org/sqlite convention and is supported in v1.28.0 — good. Just ensure the path is URL-safe (the default `./proxy.db` is fine; a configured path with `?`/spaces would need escaping, but that's an operator edge case, not blocking).
- **`internal/store` will compile but be unused** until note 06 wires it into `cmd/proxy`. That is intended (`go build ./...` still compiles the package), so it is not a dead-code problem — just confirming the plan's out-of-scope boundary is internally consistent.
- **Testing: no** is acceptable given the plan's "Done when" idempotency check (a second `Open` succeeds) is a quick manual verification; consider noting the manual check explicitly in the implementer's steps.

## Positive Notes

- Correct driver choice and rationale, preserving the hard `CGO_ENABLED=0` / single-static-binary constraint.
- Handle-leak discipline is spelled out (close `db` on any post-`sql.Open` failure), and the lazy-open trap is correctly mitigated with `Ping`.
- Schema, error-wrapping style, and package-doc conventions mirror the existing `internal/config` package faithfully.
- Clean scoping: CRUD, wiring, and migration frameworks are explicitly deferred to note 06.

---

Because of C1, this plan is not ready to implement as written. Fix Task 1's dependency pin, then it is otherwise solid.
