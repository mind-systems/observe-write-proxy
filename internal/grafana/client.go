// Package grafana implements the admin-plane's Grafana token validation: a
// single question, "is this operator's Grafana token valid right now?",
// answered by one live call to the configured Grafana instance. It is
// admin-plane-only and must never be imported by internal/proxy — the write
// plane authenticates writes against its own token store, never Grafana.
package grafana

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// validateTimeout bounds how long a single Validate call may take, so a hung
// Grafana cannot pin the calling goroutine indefinitely.
const validateTimeout = 10 * time.Second

// userPath is the fixed suffix appended to the configured Grafana base URL.
// It is the cheapest authenticated read any valid token can perform, so it
// doubles as a pure validity probe without implying a privilege check.
const userPath = "/api/user"

// ErrGrafanaURL is returned when the configured Grafana base URL is
// malformed or missing a scheme/host.
var ErrGrafanaURL = errors.New("grafana url must be an absolute URL with scheme and host")

// Client validates operator Grafana tokens for the admin plane. It holds no
// role/permission knowledge — any token Grafana accepts is treated as
// authorized to administer this proxy.
type Client struct {
	// baseURL is the resolved Grafana base URL, trailing slash trimmed once
	// at construction so request URLs never double up.
	baseURL string
	http    *http.Client
}

// NewClient validates grafanaBaseURL and returns a *Client ready to validate
// operator tokens. It fails fast on a malformed or scheme/host-less base URL
// rather than trusting an already-validated config.
func NewClient(grafanaBaseURL string) (*Client, error) {
	u, err := url.Parse(grafanaBaseURL)
	if err != nil {
		return nil, fmt.Errorf("grafana: parse grafana url %q: %w: %v", grafanaBaseURL, ErrGrafanaURL, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("grafana: grafana url %q: %w", grafanaBaseURL, ErrGrafanaURL)
	}

	return &Client{
		baseURL: strings.TrimSuffix(grafanaBaseURL, "/"),
		http: &http.Client{
			Timeout:       validateTimeout,
			CheckRedirect: refuseCrossHostRedirect,
		},
	}, nil
}

// refuseCrossHostRedirect stops net/http from following a redirect to a
// different host, so the operator's Authorization: Bearer header is never
// replayed to an unexpected origin on a 30x from a misconfigured or hostile
// Grafana.
func refuseCrossHostRedirect(req *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	if req.URL.Host != via[0].URL.Host {
		return fmt.Errorf("grafana: refusing cross-host redirect to %q", req.URL.Host)
	}
	return nil
}

// Validate reports whether token currently authenticates to Grafana.
//
// It returns (true, nil) when Grafana answers 200 — the token is valid; no
// roles, orgs, or permissions are read from the response, so any valid token
// is accepted. It returns (false, nil) when Grafana answers 401 or 403 — a
// real "not valid" verdict, not an error. Any other outcome — an unexpected
// status (5xx, 404, ...) or a transport failure (refused, DNS, TLS, timeout)
// — returns (false, err); callers must inspect err before trusting the bool,
// since "could not validate" and "invalid" mean different things and the
// caller maps err to a 5xx rather than treating it as a silent denial.
//
// The request is built with ctx so caller deadline/cancellation propagates;
// Validate starts no independent timeout beyond the Client's own transport
// timeout. The token is never emitted into a returned error or log line —
// errors describe status/transport only.
func (c *Client) Validate(ctx context.Context, token string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+userPath, nil)
	if err != nil {
		return false, fmt.Errorf("grafana: build validate request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return false, fmt.Errorf("grafana: validate request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return false, nil
	default:
		return false, fmt.Errorf("grafana: unexpected status %d validating token", resp.StatusCode)
	}
}
