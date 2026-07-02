package admin

import (
	"io/fs"
	"net/http"

	webassets "github.com/mind-systems/observe-write-proxy"
)

// NewStaticHandler returns the handler that serves the embedded admin GUI
// (web/ at the module root, embedded via webassets.FS) at "/". The page is
// public and ungated: it only collects the operator's Grafana token
// client-side, so gating it would lock the operator out before they could
// authenticate. A miss is a plain 404 from http.FileServer — there is no
// SPA index fallback.
//
// An error here means the binary was built without its static assets — a
// fatal wiring problem for the caller to report and exit on, not a runtime
// 404.
func NewStaticHandler() (http.Handler, error) {
	sub, err := fs.Sub(webassets.FS, "web")
	if err != nil {
		return nil, err
	}

	return http.FileServer(http.FS(sub)), nil
}
