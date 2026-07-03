# Plan: Fix verify.sh Grafana-token guidance

## Context
Correct `verify.sh`'s wrong prose about which Grafana token to use: a `glsa_…` service-account token is verified to return `200` from `GET /api/user` on the local stack (note 08), so it works and is the recommended credential — the current "may not resolve a user / confusing 401" warning must go. Text-only; no flow or status-code changes.

## Settings
- Testing: no
- Logging: minimal
- Docs: no

## Tasks

### Phase 1: Rewrite token guidance

- [x] **Task 1: Rewrite the header comment block on `GRAFANA_TOKEN`**
  Files: `verify.sh`
  Update the `Usage:` and `Env overrides` header comments (lines ~26–38). Change the example on line 27 away from `<grafana-user-token>` to a neutral `<grafana-token>`. Replace the trailing `GRAFANA_TOKEN must be a Grafana user/session-style token…` paragraph (lines 34–38) with guidance that any valid Grafana token is accepted — the proxy applies no role gate, validating via `GET /api/user` where any `200` means valid. State a Grafana **service-account token** (`glsa_…`, created under Administration → Service accounts → Add token) is the recommended choice, and that a user API token or session token also works. Drop the "may not resolve a user on all Grafana versions / confusing 401" warning entirely. Keep the `GRAFANA_TOKEN required, no default — see below.` override line pointing at the rewritten paragraph. Do not touch the `Expected status codes` block.

- [x] **Task 2: Rewrite the unset-`GRAFANA_TOKEN` error message** (depends on Task 1)
  Files: `verify.sh`
  Update the `if [ -z "${GRAFANA_TOKEN:-}" ]` branch (lines 66–75). Replace the echoed explanation (lines 69–73) so it matches the new header guidance: any valid Grafana token works (no role gate); a service-account token (`glsa_…`, from Administration → Service accounts → Add token) is recommended, and a user API token or session token also works. Remove the "may not resolve a user / confusing 401" wording. Keep the discipline of naming how to obtain a token without ever echoing a token value, and keep `exit 1`.

## Notes
- No code, no control flow, no status-code changes — only the two prose spots.
- Leave the four verification steps and Loki query untouched.
