# Plan Review: Local run docs (14)

**Plan:** `.ai-factory/plans/14-local-run-docs.md`
**Scope:** Documentation-only — expand `README.md` with native local-run, configuration, and SDK-pointing sections.
**Risk Level:** 🟢 Low

## Context Gates

- **Architecture** (`.ai-factory/ARCHITECTURE.md`) — present. Plan respects the two-plane model, label-blind forwarding, and single-address/path-split routing. No boundary or dependency concerns for a docs change. **PASS.**
- **Rules** (`.ai-factory/rules/base.md`) — present. Doc content matches the codified conventions: `401` for invalid/deactivated write token (rules §Error Handling), never document real token values (rules §Logging — the plan explicitly mandates a `<token>` placeholder). **PASS.**
- **Roadmap** (`.ai-factory/ROADMAP.md`) — linkage confirmed. Line 40 milestone "Local run docs" matches this plan one-to-one: four env vars with defaults, SDK pointing via `init` `headers` Bearer token, OTLP endpoint = proxy `/v1/logs`, house doc style, Docker image/e2e out of scope. **PASS.**
- **Skill-context** (`.ai-factory/skill-context/aif-review/SKILL.md`) — not present; no project-level overrides to apply.

## Verification Against the Codebase

Every factual claim in the plan was checked against source, not assumed:

- **Env var names** (`internal/config/config.go:31-34`): `PROXY_LISTEN`, `LOKI_URL`, `GRAFANA_URL`, `DB_PATH` — all four exact. ✅
- **Defaults** (`config.go:36-39`): `:4318`, `http://localhost:3100`, `http://localhost:3000`, `./proxy.db` — all four exact. ✅
- **Flag names** (`config.go:73-76`): `-listen`, `-loki-url`, `-grafana-url`, `-db-path` — exact. ✅
- **Precedence** (`config.go:58-69`): flags read env-or-default as their default, so flag > env > default holds exactly as Task 2 states. ✅
- **Loki base-URL semantics** (`config.go:19-21`, `forwarder.go:24-26,55`): `LOKI_URL` is a base URL; the proxy appends the fixed `/otlp/v1/logs` suffix when forwarding. Task 2's "must not include that path" caveat is correct and worth stating. ✅
- **Build/run targets** (`Makefile`): `make build` → `bin/proxy` (so `./bin/proxy` is the correct invocation), `make run` → `go run ./cmd/proxy`. Task 1 is accurate. ✅
- **Write endpoint** (`cmd/proxy/main.go:111`): `/v1/logs` gated by write-plane Bearer auth; forwards internally to Loki's `/otlp/v1/logs`. Task 3's endpoint distinction is correct. ✅
- **401 behavior** (`internal/proxy/auth.go:40-58`): unknown *and* deactivated tokens collapse to the same `401` (store `Lookup` filters on `active=1`). Task 3's "unknown or deactivated → 401, fails loudly" is precisely right. ✅
- **Bearer token is the proxy's own, not Grafana** (`auth.go`, plane split in `CLAUDE.md`): the SDK write path never touches Grafana. Task 3's clarification is correct and valuable. ✅

## Observations (non-blocking)

- The proxy also exposes `/healthz` (`main.go:110`) for liveness. The plan does not mention it — reasonably out of scope for a "how to run and point an SDK" doc, but a single sentence noting a health endpoint could aid operators. Optional; not a gap against the roadmap milestone.
- Task dependencies (1 → 2 → 3) are section-ordering constraints only; there is no code coupling. Sound.

## Positive Notes

- Plan pins every value the doc must state to a real source of truth rather than paraphrasing, so the implementing agent has nothing to invent.
- Correctly forbids documenting real token values and mandates the `/v1/logs` (not `/otlp/v1/logs`) endpoint — the two most common ways this doc could go wrong.
- Honors house doc style (prose, no trees, no flag table, English) and stays inside the roadmap's declared scope (no Docker image, no e2e).

No missing steps, wrong assumptions, incorrect paths, or API errors found. Being docs-only, there are no migrations or security-code concerns. The plan is solid.

PLAN_REVIEW_PASS
