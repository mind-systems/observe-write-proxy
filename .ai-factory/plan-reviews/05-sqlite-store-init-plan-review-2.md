# Plan Review 2: 05 — SQLite store init

**Plan:** `.ai-factory/plans/05-sqlite-store-init.md`
**Reviewed against:** `internal/config/config.go`, `cmd/proxy/main.go`, `go.mod`, note `05-sqlite-store-init.md`, `ARCHITECTURE.md`, `.ai-factory/rules/base.md`, installed toolchain (`go1.19.3`), and the module proxy (`proxy.golang.org`).

## Code Review Summary

**Files Reviewed:** plan + 4 code/context files
**Risk Level:** 🟢 Low

Review 1's single blocking defect (C1 — unpinned `go get` would select a Go-1.20+ sqlite and break the build) has been fully resolved. Task 1 now pins `modernc.org/sqlite@v1.28.0`, forbids cgo drivers, and spells out the rationale (Go 1.19 predates `GOTOOLCHAIN` auto-download; v1.28.0 is the last Go-1.19-compatible release). I re-verified the pin against the module proxy:

| module | version | `go` directive |
|---|---|---|
| `modernc.org/sqlite` | v1.28.0 | 1.19 ✅ |
| `modernc.org/sqlite` | v1.29.0 | 1.20 |

The pin is correct and matches the installed `go1.19.3`.

### Context Gates

- **Architecture** (`ARCHITECTURE.md`): ✅ aligned. `internal/store` is the designated "only stateful package"; it imports nothing else in-repo, honoring the dependency direction. The `tokens` schema (`name`, plaintext `token`, `active` flag, `created_at` creation metadata) matches the "Token Store Shape" canon exactly, including the reversible-deactivate-vs-hard-delete semantics `active` backs.
- **Rules** (`.ai-factory/rules/base.md`): ✅ aligned. Pure-Go driver (no cgo), single package for all store access, package name `store` and file `store.go` follow the naming rules, and errors are sentinel/`fmt.Errorf(...: %w)` wrapped as required. Commit message "Add SQLite token store initialization" satisfies the global no-prefix / sentence-case rule.
- **Roadmap**: ✅ note ties to ROADMAP → Phase 3 → Token store → "SQLite store init". Linkage present.
- **Skill-context** (`.ai-factory/skill-context/aif-review/SKILL.md`): not present — no project-specific review overrides to apply.

### Critical Issues

None. The prior blocking issue is fixed.

### Minor Notes / Suggestions (non-blocking)

- **Transitive `libc` declares `go 1.20`, not 1.19.** `modernc.org/sqlite@v1.28.0` requires `modernc.org/libc v1.29.0`, whose `go.mod` declares `go 1.20` (verified on the proxy). This does **not** break the build: Go < 1.21 treats a dependency's `go` directive as advisory and does not refuse to compile on it (the hard toolchain-version enforcement arrived with Go 1.21). Review 1 already reproduced a clean `go build` on 1.19.3. Worth being aware of only because Task 1's rationale describes v1.28.0's transitive set as clean pure-Go — accurate on the cgo point, but one indirect dep does carry a `go 1.20` line. No action needed; the plan's own guard ("if `go mod tidy` tries to raise the `go` directive, the wrong version was selected") already covers the failure mode if the module set ever shifts.
- **`mattn/go-sqlite3` appears in sqlite's own `require` block.** It is a cgo module, but it is only a test/tooling dependency of the `modernc.org/sqlite` module itself and is not on the import path reached by a blank `import _ "modernc.org/sqlite"`. `go mod tidy` in this consumer will not compile it into the binary, so `CGO_ENABLED=0` is preserved. The plan's explicit "do not add `mattn/go-sqlite3`" instruction is about direct dependencies and remains correct.
- **DSN URL-safety** (carried from review 1): the default `./proxy.db` is safe with the `?_pragma=...` query form; a configured path containing `?`/spaces/`#` would need escaping. An operator edge case, not blocking, and already outside this task's scope.
- **Manual verification is explicit.** Task 3 now names the double-`Open` idempotency check and the `CGO_ENABLED=0 go build ./...` cgo check — good, since the milestone ships no automated tests.

### Positive Notes

- Task 1 turns review 1's fix into a self-documenting rationale, so a future reader understands *why* the version is pinned and what raising it would cost — this prevents the regression rather than just patching it.
- Handle-leak discipline (close `db` on any post-`sql.Open` failure), the lazy-`sql.Open` + `Ping` trap, connection-scoped pragmas via DSN, and `IF NOT EXISTS`-only idempotency are all correctly specified.
- Scoping is clean and consistent with the note: CRUD/lookup, `cmd/proxy` wiring, and migration frameworks are all deferred to note 06, and `config.DBPath` / default `./proxy.db` already exist in the codebase so no config work is smuggled in.
- Conventions mirror the existing `internal/config` package faithfully (package doc, exported-identifier docs, wrapped errors).

---

The plan is solid and ready to implement. The one blocking issue from review 1 is resolved; remaining items are informational only.

PLAN_REVIEW_PASS
