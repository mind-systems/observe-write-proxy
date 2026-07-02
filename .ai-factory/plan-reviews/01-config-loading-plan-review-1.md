# Plan Review: 01 — Config loading

## Code Review Summary

**Files Reviewed:** plan `01-config-loading.md` + spec note `notes/01-config-loading.md`, cross-checked against ROADMAP, ARCHITECTURE, rules/base.md, go.mod, current tree
**Risk Level:** 🟢 Low

### Context Gates

- **Architecture** — WARN (non-blocking). `ARCHITECTURE.md`'s folder-structure template lists `cmd/proxy`, `internal/{proxy,admin,store,grafana}` but does **not** list `internal/config`. This is not a real conflict: the doc explicitly says "Concrete package layout is a starting point — refine as the service takes shape," and the ROADMAP milestone authorizes `internal/config` by name. Dependency rules are respected — config is a pure leaf importing nothing else in the repo. No action required; optionally add `config/` to the ARCHITECTURE template in a later pass.
- **Rules** (`rules/base.md`) — PASS. Plan honours house conventions: `snake_case.go` filename (`config.go`), `PascalCase` exports / `camelCase` unexported (`envOr`), sentinel `ErrXxx` errors wrapped with `fmt.Errorf("...: %w", …)`, pure-Go (no new deps, no cgo). Logging rule "never log token values" is not relevant here (no secrets in config).
- **Roadmap** — PASS. Plan maps 1:1 to Phase 1 → "Config loading": same struct fields, same env var names, same canon defaults (`:4318`, `http://localhost:3100`, `http://localhost:3000`, `./proxy.db`), and the same deferred-join decision (`<LokiURL>/otlp/v1/logs` joined later in `proxy`, not stored). Consistent with the spec note.

### Critical Issues

None. No missing steps, no wrong codebase assumptions, no incorrect file paths, and no API misuse that would block implementation.

- File path `internal/config/config.go` is correct for module `github.com/mind-systems/observe-write-proxy`; the `internal/` tree does not yet exist and will be created — fine.
- Go version assumption verified: toolchain is `go1.19.3`. The spec's downstream reliance on `url.URL.JoinPath` (task 03) is valid — `JoinPath` landed in Go 1.19. This task uses only `net/url.Parse` + scheme/host checks, all pre-1.19 stable.
- Precedence logic (flag > env > default via env-seeded flag defaults, reading env **before** flag definition) is correct for every case (flag-set, env-only, neither). The `flag`-reads-default-at-definition-time watch note is captured accurately.
- Validation rules are sound, including the genuinely non-obvious `localhost:3100` case (`url.Parse` yields `Scheme="localhost"`, `Opaque="3100"`, empty `Host`) — the `Scheme != "" && Host != ""` guard is the correct catch.
- Config package is standalone and compiles without a caller; `main` wiring is correctly deferred to task 02.

### Minor / Optional Notes (non-blocking)

1. **`Load()` parses `os.Args[1:]` via its own `FlagSet`.** Correct and self-contained for production (called once from `main`). Two things to keep in mind at implementation time, neither blocking:
   - Choose the `FlagSet` error mode deliberately (`flag.ContinueOnError` returning the error up, vs `flag.ExitOnError`). `ContinueOnError` composes better with the fail-fast "return `nil, err`" contract this package promises.
   - If a future task ever adds unit tests, calling `Load()` under `go test` would parse the test binary's args. Testing is intentionally off here, so this is only a forward note.
2. **`envOr` empty-string semantics.** Using `os.Getenv` treats an explicitly-set-empty var (`PROXY_LISTEN=`) the same as unset (falls back to canon default). That is the sensible behaviour and any truly-empty value would be caught by validation anyway — just confirm the intent (Getenv) rather than `LookupEnv`.
3. **Testing = no for a resolution/validation-heavy package.** Per the project's test philosophy (test only silent-failure surfaces), most of this fails loudly (validation returns errors; a wrong URL 502s at forward time). A precedence bug is the one surface that could resolve to a plausible-but-wrong value silently. Not a blocker given the explicit "Testing: no" setting — flagged only so the decision is conscious.

### Positive Notes

- Excellent single-source-of-truth discipline: `LokiURL` stored as base only, with the slash-normalization concentrated at one downstream `JoinPath` — the plan explicitly forbids trimming/rewriting the trailing slash here, avoiding the classic two-places-massage-the-slash bug.
- Fail-fast contract is precise: return `nil` + a named, wrapped sentinel error, "leaking no partially-initialized config."
- The env-before-flags ordering subtlety is called out explicitly rather than left to be rediscovered.
- Task dependencies (1 → 2 → 3) are correct and the scope boundary against task 02 (binding/exit/server) is clean.

PLAN_REVIEW_PASS
