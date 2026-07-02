// Command proxy is the observe-write-proxy entry point: it owns the
// sequence config → mux → server → shutdown, wiring the write plane and
// admin plane onto a single HTTP server and draining in-flight requests on
// SIGINT/SIGTERM.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mind-systems/observe-write-proxy/internal/config"
	"github.com/mind-systems/observe-write-proxy/internal/proxy"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fwd, err := proxy.NewForwarder(cfg.LokiURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	mux := newMux(proxy.NewHandler(fwd))

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		// ListenAndServe returns http.ErrServerClosed on a clean Shutdown;
		// that is not a failure. Any other error is fatal.
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	// Restore default signal handling before draining: a second SIGINT
	// during a hung Shutdown must force-kill the process rather than just
	// re-cancel the already-cancelled ctx.
	stop()

	// Parent on context.Background(), not ctx: ctx is already Done (it's
	// what just unblocked the select above), so deriving from it would make
	// shutdownCtx cancelled from birth and Shutdown would return
	// immediately, dropping in-flight requests.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newMux builds the route table, planes split by path per canon. /healthz
// and /v1/logs are real handlers; every other route is a placeholder that
// later tasks replace.
func newMux(h *proxy.Handler) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", handleHealthz)
	mux.Handle("/v1/logs", h)
	mux.HandleFunc("/api/tokens", handleNotImplemented)
	mux.HandleFunc("/api/tokens/", handleNotImplemented)
	mux.HandleFunc("/", handleNotImplemented)

	return mux
}

// handleHealthz reports that this process is alive. It must not depend on
// cfg, the store, or any upstream — liveness, not readiness.
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// handleNotImplemented is the placeholder for routes wired by later tasks.
// It answers 501 rather than a silent 200, so a half-wired build is
// obvious.
func handleNotImplemented(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented yet", http.StatusNotImplemented)
}
