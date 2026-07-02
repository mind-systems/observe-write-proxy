package proxy

import (
	"io"
	"net/http"
)

// Handler is the write plane's HTTP entry point: it owns a *Forwarder — its
// only dependency — and relays Loki's response verbatim. It satisfies
// http.Handler (rather than exposing a bare http.HandlerFunc) so a later
// task can wrap it with auth middleware without editing it. No auth is
// enforced here; that gap is closed by the middleware task.
type Handler struct {
	fwd *Forwarder
}

// NewHandler returns a *Handler that forwards accepted requests through fwd.
func NewHandler(fwd *Forwarder) *Handler {
	return &Handler{fwd: fwd}
}

// ServeHTTP guards the method, then streams the request body straight to
// Loki via Forward and relays the upstream status and body unchanged. It
// never reads, buffers, or inspects the body — streaming plus the
// label-blind guarantee — and never invents a status: Loki's 2xx and its
// 4xx/5xx are both passed through as-is.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	contentType := r.Header.Get("Content-Type")
	contentEncoding := r.Header.Get("Content-Encoding")

	resp, err := h.fwd.Forward(r.Context(), r.Body, contentType, contentEncoding)
	if err != nil {
		http.Error(w, "failed to forward to loki", StatusForError(err))
		return
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
