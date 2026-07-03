## Code Review Summary

**Files Reviewed:** 1 plan (`17-server-side-name-validation-on-create.md`), cross-checked against `internal/admin/tokens.go`, `internal/store/token.go`, `web/app.js`, `.ai-factory/notes/17-create-name-validation.md`, and `.ai-factory/ROADMAP.md`
**Risk Level:** 🟢 Low

### Context Gates
- **Architecture** — WARN: no `.ai-factory/ARCHITECTURE.md` boundary conflict. The plan keeps validation as a handler concern and explicitly leaves `store.Create` a dumb insert, respecting the admin/store separation. No violation.
- **Rules** — no `RULES.md` present; project no-echo discipline (never log request body) is explicitly honored by the plan.
- **Roadmap** — PASS: the plan maps 1:1 to the open ROADMAP line "Server-side name validation on create" (Phase 10 — Post-review fixes) and its `Spec:` note `.ai-factory/notes/17-create-name-validation.md`, which in turn traces to the note-10 token-management contract. Milestone linkage is intact.

### Critical Issues
None.

### Verification of plan claims against the codebase
- **Insertion point is correct.** `Collection`'s `http.MethodPost` case (`tokens.go:84`) decodes the body at line 88–91, then calls `h.store.Create(body.Name)` at line 93. The guard belongs between these two points exactly as the plan states.
- **`strings` already imported.** Confirmed at `tokens.go:7` (used by `Item` for `TrimPrefix`), so no import change — as claimed.
- **`store.Create(name string)` signature matches** (`internal/store/token.go:48`); no store-layer change needed and none proposed. No DB schema change, so **no migration required** — correctly reflected (this is a pure request-validation guard).
- **GUI already guards client-side.** `web/app.js:221–223` trims and blocks an empty name, so this change only affects direct-API callers and leaves the GUI flow unchanged — matches the plan and spec.
- **Error semantics correct.** `400` for the empty/whitespace case is distinct from the `500` returned on a `store.Create` failure (`tokens.go:95`), preserving the client-vs-server error distinction the spec calls out.
- **Trim-before-check** covers both `{"name":""}` and `{"name":"   "}`, satisfying the spec's "Done when" acceptance.

### Minor Notes (non-blocking)
- **Testing: no** is consistent with the project's test philosophy — an added `400` guard is a loud, direct-response surface, and the change is a single conditional. Acceptable given the spec's explicit acceptance criteria; a future request-level test would be a nice-to-have, not a gap in this plan.
- The plan text embeds the exact code line to add. That is fine here given the triviality, and it matches the spec's own wording verbatim, reducing implementer ambiguity.

### Positive Notes
- Precise file path, insertion point, and rationale — the implementer has no fantasy holes to fill.
- Explicitly preserves the store's separation of concerns and the handler's no-echo logging discipline.
- Correctly scopes the change to the direct-API gap and confirms the `201` happy path stays unchanged.

PLAN_REVIEW_PASS
