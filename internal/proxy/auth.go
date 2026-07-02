package proxy

import (
	"net/http"
	"strings"

	"github.com/mind-systems/observe-write-proxy/internal/store"
)

// tokenLookup is the minimal store capability the write-plane auth gate
// needs. It exists so this package depends only on the shape it uses, not on
// *store.Store's full surface, while still honoring the proxy→store
// dependency rule. *store.Store satisfies it via Lookup.
type tokenLookup interface {
	Lookup(token string) (store.Token, bool, error)
}

// authMiddleware guards next with a Bearer-token check against lookup,
// performed fresh on every request — no caching — so a token deactivated in
// the store fails on the very next write.
type authMiddleware struct {
	lookup tokenLookup
	next   http.Handler
}

// NewAuthMiddleware returns an http.Handler that gates next behind a Bearer
// token check. It wraps the existing write-path *Handler without modifying
// it, so the forwarding logic in handler.go stays untouched.
func NewAuthMiddleware(lookup tokenLookup, next http.Handler) http.Handler {
	return &authMiddleware{lookup: lookup, next: next}
}

// ServeHTTP extracts the Bearer credential, looks it up, and either
// delegates to next (valid, active token) or rejects the request. A missing
// or malformed header and an unknown or inactive token all produce the same
// generic 401 — no oracle that would let a caller distinguish "no such
// token" from "token exists but is inactive". A store error is a distinct
// 500 so a backend hiccup never masquerades as an auth failure. The request
// body is never read here; the gate only ever inspects headers.
func (m *authMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// store.Lookup's active=1 filter means ok is only true for a row that
	// exists and is active — "unknown" and "inactive" collapse into the
	// same ok=false, err=nil result, giving the identical 401 below.
	_, ok, err := m.lookup.Lookup(token)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	m.next.ServeHTTP(w, r)
}

// bearerToken extracts the credential from an Authorization header value of
// the form "Bearer <token>". The scheme match is case-insensitive; the
// credential is trimmed of surrounding whitespace. ok is false for a missing
// header, a header with no space-separated scheme, a non-Bearer scheme, or
// an empty credential after trimming.
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
