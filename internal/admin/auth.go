// Package admin implements the admin plane: the Grafana-authenticated
// management API and GUI that mint, list, toggle, and delete write-plane
// tokens. It never imports internal/proxy — the two auth planes are wired
// independently by cmd/proxy and must not share code.
package admin

import (
	"context"
	"net/http"
	"strings"
)

// grafanaValidator is the minimal Grafana capability this gate needs. It
// exists so admin depends only on the shape it uses, not on *grafana.Client's
// full surface — mirrors the tokenLookup narrow-interface pattern in
// internal/proxy/auth.go. *grafana.Client satisfies it via Validate.
type grafanaValidator interface {
	Validate(ctx context.Context, token string) (bool, error)
}

// RequireGrafana returns middleware that gates next behind a live Grafana
// Bearer-token check, performed fresh on every request — no session, cookie,
// or cache — so a token revoked in Grafana stops working on the very next
// admin call.
func RequireGrafana(v grafanaValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			valid, err := v.Validate(r.Context(), token)
			if err != nil {
				http.Error(w, "grafana unreachable", http.StatusServiceUnavailable)
				return
			}
			if !valid {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// bearerToken extracts the credential from an Authorization header value of
// the form "Bearer <token>". The scheme match is case-insensitive; the
// credential is trimmed of surrounding whitespace. ok is false for a missing
// header, a header with no space-separated scheme, a non-Bearer scheme, or
// an empty credential after trimming. Copied from internal/proxy/auth.go
// rather than shared — the two planes never import each other.
func bearerToken(header string) (string, bool) {
	if header == "" {
		return "", false
	}

	scheme, credential, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "bearer") {
		return "", false
	}

	credential = strings.TrimSpace(credential)
	if credential == "" {
		return "", false
	}

	return credential, true
}
