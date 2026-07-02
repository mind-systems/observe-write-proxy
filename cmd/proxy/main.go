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

	"github.com/mind-systems/observe-write-proxy/internal/admin"
	"github.com/mind-systems/observe-write-proxy/internal/config"
	"github.com/mind-systems/observe-write-proxy/internal/grafana"
	"github.com/mind-systems/observe-write-proxy/internal/proxy"
	"github.com/mind-systems/observe-write-proxy/internal/store"
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

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer st.Close()

	gclient, err := grafana.NewClient(cfg.GrafanaURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	static, err := admin.NewStaticHandler()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	mux := newMux(st, proxy.NewHandler(fwd), admin.NewTokenHandlers(st), admin.RequireGrafana(gclient), static)

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
// and /v1/logs are real handlers — /v1/logs gated by the write-plane Bearer
// auth middleware backed by st — and /api/tokens[/] serve the real
// management API (th), gated by gate, the admin-plane Grafana auth
// middleware. / serves the embedded static GUI (static) and stays ungated:
// the page only collects the operator's Grafana token, so gating it would
// lock the operator out. ServeMux's longest-prefix matching keeps the more
// specific patterns above ahead of this catch-all regardless of
// registration order.
func newMux(st *store.Store, h *proxy.Handler, th *admin.TokenHandlers, gate func(http.Handler) http.Handler, static http.Handler) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", handleHealthz)
	mux.Handle("/v1/logs", proxy.NewAuthMiddleware(st, h))
	mux.Handle("/api/tokens", gate(http.HandlerFunc(th.Collection)))
	mux.Handle("/api/tokens/", gate(http.HandlerFunc(th.Item)))
	mux.Handle("/", static)

	return mux
}

// handleHealthz reports that this process is alive. It must not depend on
// cfg, the store, or any upstream — liveness, not readiness.
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
