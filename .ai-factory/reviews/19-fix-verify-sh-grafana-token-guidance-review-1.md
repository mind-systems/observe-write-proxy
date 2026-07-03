# Review: Fix verify.sh Grafana-token guidance

## Scope
Text-only change to `verify.sh`: two prose spots (the header comment block and the unset-`GRAFANA_TOKEN` error message) rewritten to say any valid Grafana token — including a `glsa_…` service-account token — is accepted, dropping the incorrect "may not resolve a user / confusing 401" warning.

## Verification
- Reviewed `git status` and `git diff HEAD`. Only `verify.sh` has runtime relevance; the other staged changes are `.ai-factory/` planning artifacts.
- Read `verify.sh` in full. The diff touches only comment lines (24–38) and the `echo … >&2` lines inside the `if [ -z "${GRAFANA_TOKEN:-}" ]` branch (69–73).
- Confirmed against the spec (note 08 / note 19): the proxy's `Validate` uses `GET /api/user` with no role gate, and a `glsa_…` service-account token returns `200`. The new guidance matches this exactly.

## Findings
- **Correctness:** No control flow, status codes, curl invocations, or the four verification steps were altered. `set -euo pipefail` and `exit 1` remain intact. The rewrite is confined to prose.
- **Security:** No token value is echoed anywhere; the messages only describe how to obtain a token (Administration → Service accounts → Add token). Never-log-the-token discipline preserved.
- **Consistency:** The header block and the runtime error message now carry the same wording, and both align with `Validate`'s no-role-gate policy. The `GRAFANA_TOKEN required, no default — see below.` override line still points at the rewritten paragraph.
- The `Usage:` example changed from `<grafana-user-token>` to the neutral `<grafana-token>`, consistent with the accept-any-valid-token message.

No bugs, security issues, or correctness problems.

REVIEW_PASS
