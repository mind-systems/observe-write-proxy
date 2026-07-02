# Code Review: 08 — Grafana validation client

**Scope:** `internal/grafana/client.go` (only new/changed code file in the diff; other changed files are planning artifacts).
**Build/vet:** `go build ./internal/grafana/` and `go vet ./internal/grafana/` both clean.
**Verdict:** No correctness, security, or runtime defects found. Two non-blocking observations below.

## Summary

The implementation faithfully matches the plan and spec (note 08). It:
- Validates the base URL for scheme+host and fails fast with a wrapped `ErrGrafanaURL`, mirroring `config.isValidBaseURL` / `proxy.NewForwarder`.
- Trims the trailing slash once at construction, so `baseURL + "/api/user"` never double-slashes.
- Builds the request with `NewRequestWithContext`, propagating caller deadline/cancellation.
- Maps `200 → (true,nil)`, `401/403 → (false,nil)`, everything else + transport error → `(false, err)`.
- Refuses cross-host redirects so the operator `Authorization: Bearer` is never replayed to another origin.
- Never emits the token into any error string or log.

## Correctness checks performed

- **Body-close on the redirect-error path.** When `CheckRedirect` returns an error, `Do` returns the previous response *with its Body already closed* (confirmed via `go doc net/http.Client.CheckRedirect`), and the code returns before reaching `defer resp.Body.Close()`. On a transport error `resp` is nil. So the `err != nil` early return leaks nothing — correct.
- **Field name `http` vs package `http`.** The struct field `http` does not shadow the imported package; field access (`c.http`) and package identifier (`http.MethodGet`) live in separate namespaces. Compiles and vets clean.
- **Cross-host comparison uses `via[0]`** (the original request), so a chained A→A→B redirect is still refused at the B hop. `Host` includes the port, so a port change is also refused — a conservative, correct choice.
- **Token confidentiality.** No code path formats `token` into an error or log. Errors carry only status code / transport cause / redirect host. Matches the canon guard.

## Observations (non-blocking)

1. **Custom `CheckRedirect` removes Go's default 10-redirect cap.** Per the stdlib docs, the 10-redirect limit only applies when `CheckRedirect == nil`. Because this client sets a custom `CheckRedirect` that only refuses *cross-host* hops, a Grafana that issues a same-host redirect loop would be followed indefinitely — bounded in practice only by the 10s `Timeout`. Risk is low (admin plane, operator-configured trusted Grafana, timeout backstop), but a redirect-depth guard (`if len(via) >= 10 { return err }`) would make the failure fast and explicit instead of a 10s hang. Optional.

2. **Response body is closed but never drained.** For keep-alive connection reuse, an undrained body can prevent the underlying TCP connection from being reused. `/api/user` responses are tiny and the admin plane is low-frequency, so this is a micro-optimization, not a defect. Optional: `io.Copy(io.Discard, resp.Body)` before close.

Neither observation blocks the milestone; both are optional hardening on a low-frequency, trusted-peer path.

REVIEW_PASS
