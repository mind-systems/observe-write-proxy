// Package proxy implements the write plane's core: a label-blind forwarder
// that streams an incoming OTLP/HTTP log body straight through to Loki. It
// never reads, buffers, decompresses, or parses the body, and it never
// rewrites OTLP attributes or Loki labels — authentication and pass-through
// forwarding are its only jobs; everything else belongs upstream (the
// SDKs) or downstream (Loki itself).
package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// forwardTimeout bounds how long a single forward to Loki may take, so a
// hung Loki cannot pin a goroutine indefinitely.
const forwardTimeout = 30 * time.Second

// lokiLogsPath is the fixed suffix appended to the configured Loki base URL
// to reach its OTLP log-ingestion endpoint.
const lokiLogsPath = "/otlp/v1/logs"

// ErrLokiURL is returned when the configured Loki base URL is malformed or
// missing a scheme/host.
var ErrLokiURL = errors.New("loki url must be an absolute URL with scheme and host")

// Forwarder streams OTLP/HTTP log bodies to Loki unchanged. It is
// label-blind: it must never import internal/admin or internal/grafana, and
// it must never parse or rewrite the forwarded body.
type Forwarder struct {
	// target is the fully resolved forward URL, computed once at
	// construction so the hot path builds no URLs.
	target string
	client *http.Client
}

// NewForwarder validates lokiBaseURL and returns a *Forwarder ready to
// stream requests to Loki's OTLP endpoint. It fails fast on a malformed or
// scheme/host-less base URL rather than trusting an already-validated
// config.
func NewForwarder(lokiBaseURL string) (*Forwarder, error) {
	u, err := url.Parse(lokiBaseURL)
	if err != nil {
		return nil, fmt.Errorf("proxy: parse loki url %q: %w: %v", lokiBaseURL, ErrLokiURL, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("proxy: loki url %q: %w", lokiBaseURL, ErrLokiURL)
	}

	target := strings.TrimSuffix(lokiBaseURL, "/") + lokiLogsPath

	return &Forwarder{
		target: target,
		client: &http.Client{Timeout: forwardTimeout},
	}, nil
}

// Forward streams body to Loki's OTLP log endpoint unchanged, setting only
// Content-Type and (when present) Content-Encoding. It never reads,
// buffers, decompresses, or parses body, and it copies no other headers —
// no hop-by-hop headers are forwarded; the client manages transport
// framing.
//
// On success it returns the raw *http.Response from Loki and a nil error;
// the caller owns relaying status and body verbatim (including 4xx/5xx)
// and is responsible for closing resp.Body.
//
// On a transport failure it returns a nil response and a wrapped error;
// pass the error to StatusForError to determine whether to answer 502 or
// 504. Forward never retries, to avoid duplicate ingestion.
func (f *Forwarder) Forward(ctx context.Context, body io.Reader, contentType, contentEncoding string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.target, body)
	if err != nil {
		return nil, fmt.Errorf("proxy: build forward request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	if contentEncoding != "" {
		req.Header.Set("Content-Encoding", contentEncoding)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("proxy: forward to loki: %w", err)
	}

	return resp, nil
}

// timeouter is satisfied by net.Error and *url.Error, both of which report
// whether the failure they wrap was a timeout.
type timeouter interface {
	Timeout() bool
}

// StatusForError classifies a non-nil error returned by Forward into the
// HTTP status the caller should answer with: 504 when the failure was a
// timeout or deadline, 502 for every other transport failure (connection
// refused, DNS, TLS, ...). It uses errors.As/errors.Is rather than a bare
// type assertion so the classification still works through Forward's
// fmt.Errorf("...: %w", ...) wrapping.
func StatusForError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout
	}
	var te timeouter
	if errors.As(err, &te) && te.Timeout() {
		return http.StatusGatewayTimeout
	}
	return http.StatusBadGateway
}
