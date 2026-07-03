# 19 — Fix verify.sh Grafana-token guidance

**Task:** ROADMAP → Phase 10 — Post-review fixes → "Fix verify.sh Grafana-token guidance"
**Depth:** low

## Goal

`verify.sh`'s guidance about which Grafana token to use is wrong and must be corrected. The script header comment and the `GRAFANA_TOKEN`-unset error both warn that a service-account token "may not resolve a user on all Grafana versions and would surface as a confusing 401" and tell the operator to use a user/session token instead. This contradicts the proxy's own verified behaviour.

## Why it's wrong

- The proxy validates admin tokens via `GET /api/user` with no role gate: any `200` means valid (`internal/grafana/client.go`, `Validate`).
- Note 08 in this repo records the verification: a `glsa_…` service-account token returns `200` from `GET /api/user` on the local stack (identity `sa-…`, `orgId 1`); a bogus token returns `401`. So a service-account token is exactly what works — and is the same credential pasted into the GUI's single field.
- The script therefore steers the operator away from a credential that is known to work, for no reason.

## Design

- Edit `verify.sh` only — no code, no flow change. Rewrite two spots:
  1. The header comment block explaining `GRAFANA_TOKEN`.
  2. The `if [ -z "${GRAFANA_TOKEN:-}" ]` error message.
- New guidance: any valid Grafana token is accepted — the proxy applies no role gate. A Grafana **service-account token** (`glsa_…`, created under Administration → Service accounts → Add token) is the recommended choice; a user API token or session token also works. Drop the "may not resolve a user / confusing 401" warning entirely.

## Edge cases / watch

- Keep the never-log-the-token discipline: the message names how to obtain a token, never echoes one.
- Do not touch the four verification steps or the expected status codes — only the prose guidance changes.

## Done when

- `verify.sh`'s header and unset-token error describe a `glsa_…` service-account token as a valid, recommended credential and no longer warn against it.
- Running `verify.sh` with a service-account token completes step 1 (`mint`) with `201`, matching the proxy's actual behaviour.
