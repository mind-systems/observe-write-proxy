## Code Review Summary

**Files Reviewed:** 1 plan (`18-patch-active-must-reject-a-missing-field.md`), verified against `internal/admin/tokens.go`, `internal/store/token.go`, the spec note, and the ROADMAP contract line.
**Risk Level:** 🟢 Low

### Context Gates

- **Roadmap (`.ai-factory/ROADMAP.md`):** ✅ Matches. The plan corresponds exactly to the `[ ]` contract line "PATCH active must reject a missing field" (Phase 10 — Post-review fixes), which prescribes the `*bool` + `400`-on-nil change in `internal/admin/tokens.go`, Item PATCH branch. `Spec:` tag resolves to `.ai-factory/notes/18-patch-active-required.md`.
- **Spec note (`.ai-factory/notes/18-patch-active-required.md`):** ✅ Present and fully consistent with the plan — same decode-target pointer change, same nil → `400` behavior, same preservation of `404`/`500`/`200` paths, same malformed-JSON `400`. The note additionally flags that `web/app.js` always sends an explicit `active` and is therefore unaffected; the plan correctly scopes it out.
- **Architecture (`.ai-factory/ARCHITECTURE.md`):** ✅ No boundary impact. Change is internal to the admin handler; the label-blind write plane and the two-auth-plane split are untouched.
- **Rules/skill-context:** No `.ai-factory/skill-context/aif-review/SKILL.md` present — no project-specific overrides to apply.

### Correctness Verification

- **Line references accurate.** `TokenHandlers.Item` PATCH branch sits at lines 121–138; the current decode target is `struct{ Active bool `json:"active"` }` (lines 122–124), exactly as the plan describes.
- **API usage correct.** `store.SetActive(id string, active bool)` (confirmed in `internal/store/token.go:124`) takes a `bool`, so `*body.Active` after the nil check is the correct argument. `store.ErrNotFound` (line 15) → `404` mapping is real and preserved.
- **Root cause is real.** With a plain `bool`, `{}` decodes to `Active == false` and silently deactivates the token with `200`. The `*bool` sentinel is the correct idiom to distinguish "absent" (nil) from "present and false" (`&false`).
- **No migration needed.** No schema or store change — purely request-decoding logic. Correctly reflected by Docs: no, Testing: no.
- **Decode-error branch preserved.** Malformed JSON continues to hit the existing `err != nil` → `400` branch (line 125–128); the new nil check runs only after a successful decode. Ordering is right.

### Critical Issues

None.

### Minor Notes (non-blocking)

- The plan sets Testing: no. A `verify.sh` E2E path exists in this repo, and the behavior being fixed (silent deactivation on `{}`) is exactly the class of bug that fails silently rather than loudly. This is a judgment call and acceptable for a low-depth single-line fix, but if the change is ever regressed it would pass compile/DI and only surface as wrong runtime state — worth a one-line curl assertion in `verify.sh` if convenient. Not a blocker.
- The plan does not restate that `web/app.js` is unaffected; the spec note covers it, so no action needed.

### Positive Notes

- Precise, single-file, single-task scope with accurate line anchors and correct instruction to leave the DELETE branch and `Collection` handler untouched.
- Correctly preserves every existing status-code path (`400` malformed, `404` not-found, `500` internal, `200` success) and only adds the new `400`-on-absent branch.
- Fully traceable through the two-tier roadmap (contract line → spec note → plan) with no drift.

PLAN_REVIEW_PASS
