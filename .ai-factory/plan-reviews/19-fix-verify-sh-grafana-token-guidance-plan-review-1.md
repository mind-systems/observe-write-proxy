## Code Review Summary

**Files Reviewed:** 1 plan (`19-fix-verify-sh-grafana-token-guidance.md`) against `verify.sh`, `internal/grafana/client.go`, note 08, note 19, ROADMAP
**Risk Level:** 🟢 Low

### Context Gates
- **Roadmap:** WARN-free. The plan traces cleanly to `ROADMAP.md` Phase 10 → "Fix verify.sh Grafana-token guidance", whose contract line points at `Spec: .ai-factory/notes/19-verify-token-guidance.md`. Plan scope matches the spec note exactly (rewrite two prose spots, no flow change).
- **Architecture:** N/A. `.ai-factory/ARCHITECTURE.md` present but this is a doc/prose change to a shell script — no module boundary, dependency, or auth-plane concern touched.
- **Rules:** No `.ai-factory/RULES.md` present — gate skipped.
- **Skill-context:** No `.ai-factory/skill-context/aif-review/SKILL.md` present — no project overrides to apply.

### Correctness of the plan's premise
The plan's core claim is verified against the actual code, not just asserted:
- `internal/grafana/client.go` `Validate` returns `(true, nil)` on any `200` from `GET /api/user`, reads no role/org/permission — confirming the "no role gate, any valid token works" guidance the plan wants to write.
- Note 08 records the live-stack verification: a `glsa_…` service-account token returns `200` (identity `sa-…`, `orgId 1`); a bogus token returns `401`. So the current warning steering operators *away* from service-account tokens is genuinely wrong, exactly as the plan states.

### Line-reference accuracy (checked against current `verify.sh`)
All cited line ranges are correct:
- Task 1: header comment block lines 26–38; example on line 27 is `GRAFANA_TOKEN=<grafana-user-token> ./verify.sh` ✓; trailing paragraph to replace is lines 34–38 ✓; the `GRAFANA_TOKEN required, no default — see below.` override line is line 32 ✓; `Expected status codes` block (lines 15–24) correctly flagged as do-not-touch ✓.
- Task 2: `if [ -z "${GRAFANA_TOKEN:-}" ]` branch lines 66–75; echoed explanation lines 69–73; `exit 1` at line 74 ✓.

### Critical Issues
None. No missing steps, no wrong codebase assumptions, no missing migration (text-only shell edit — no schema, no store, no API surface), no security regression. The plan explicitly preserves the never-echo-the-token discipline and leaves the four verification steps, status codes, and Loki query untouched — matching the spec note's "watch" items.

### Minor Notes (non-blocking)
- Both tasks and the spec note say to reference the token-creation path as "Administration → Service accounts → Add token". This is the correct Grafana UI path for current versions; worth the implementer keeping it verbatim across both spots so the header and the error message stay consistent.
- Task dependency ordering (Task 2 depends on Task 1) is stated but functionally irrelevant since both edits are independent regions of the same file — no harm, and it correctly signals the two messages should read consistently.

### Positive Notes
- Tight, well-scoped plan: the premise is grounded in verified runtime behaviour (note 08 + `Validate` source), not speculation.
- Correctly identifies both stale copies of the wrong guidance (header comment *and* the unset-token error) — a partial fix leaving one in place would have been the likely failure mode, and the plan closes both.
- Explicitly fences off the risky-to-touch regions (status-code table, verification steps, Loki query), keeping a doc fix from drifting into a behaviour change.

PLAN_REVIEW_PASS
