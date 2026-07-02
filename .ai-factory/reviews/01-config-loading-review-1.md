# Code Review: 01 — Config loading

**Scope:** `internal/config/config.go` (new). Plan/spec: `.ai-factory/plans/01-config-loading.md`, `.ai-factory/notes/01-config-loading.md`.
**Build/vet:** `go build ./...` and `go vet ./...` both pass.
**Risk:** 🟢 Low — single pure-leaf package, no I/O, no state.

## What was checked

- **Precedence (flag > env > default).** Env values are read via `envOr` into the per-flag defaults *before* `fs.Parse`, so `flag`'s definition-time default capture yields the intended precedence: a CLI-passed flag wins, else the env value, else the canon default. Correct for flag-set / env-only / neither. The env-before-flags ordering subtlety (spec watch note) is honored and commented.
- **Env semantics.** `envOr` uses `os.LookupEnv`, so an explicitly-set-empty var is passed through as `""` rather than silently reverting to default — and any empty listen/db value is then caught by `validate`. Safe.
- **Validation / fail-fast.** `ListenAddr` and `DBPath` rejected only on emptiness (bare `:4318` stays valid — binding deferred to task 02). `LokiURL`/`GrafanaURL` require `url.Parse` success **and** non-empty `Scheme` and `Host`. Verified the non-obvious case: `url.Parse("localhost:3100")` returns no error with an empty `Host` (`Scheme="localhost"`, `Opaque="3100"`), so the explicit `Host != ""` check is the real guard — implemented correctly. Empty `raw`, `"http://"`, and `"http:///path"` are all correctly rejected (empty host).
- **No partial config leak.** On any validation failure `Load` returns `nil, err`; the `*Config` is only returned after `validate` passes.
- **Single normalization point.** `LokiURL` is stored verbatim with no trailing-slash trimming, per the deferred-`JoinPath` decision — no double-massage bug introduced here.
- **House conventions.** `snake_case.go` filename, `PascalCase` exports / `camelCase` unexported (`envOr`, `validate`, `isValidBaseURL`), sentinel `ErrXxx` values wrapped with `fmt.Errorf("...: %w", …)`, pure-Go with no new dependencies (module stays cgo-free). No token/secret logging surface.
- **Scope boundary.** Package compiles standalone with no caller; server binding, exit codes, and signal handling correctly deferred to task 02. `LokiURL` consumers (the `/otlp/v1/logs` join) correctly deferred to `internal/proxy`.

## Findings

None. No correctness, security, or runtime defects. Prior plan-review non-blocking notes (`-h`/`flag.ErrHelp` exiting non-zero under `ContinueOnError`; `os.Args` parsing under a hypothetical future test binary) are expected behavior within this milestone's scope, not bugs — testing is intentionally off for this package.

REVIEW_PASS
