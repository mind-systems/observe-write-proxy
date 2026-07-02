// Package config resolves the proxy's runtime settings from flags,
// environment variables, and canon defaults, validating them up front so
// misconfiguration fails at startup rather than as a silent forwarding
// failure later.
package config

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
)

// Config holds the proxy's validated runtime settings.
type Config struct {
	// ListenAddr is the address the HTTP server binds to, e.g. ":4318".
	ListenAddr string
	// LokiURL is Loki's base URL — deliberately not the /otlp/v1/logs
	// endpoint itself; the join happens later in internal/proxy.
	LokiURL string
	// GrafanaURL is the base URL of the Grafana instance used to validate
	// admin-plane operator tokens.
	GrafanaURL string
	// DBPath is the filesystem path to the SQLite token store.
	DBPath string
}

// Canon env var names and defaults.
const (
	envListen  = "PROXY_LISTEN"
	envLoki    = "LOKI_URL"
	envGrafana = "GRAFANA_URL"
	envDBPath  = "DB_PATH"

	defaultListen  = ":4318"
	defaultLoki    = "http://localhost:3100"
	defaultGrafana = "http://localhost:3000"
	defaultDBPath  = "./proxy.db"
)

// Sentinel errors, each naming the setting that failed validation.
var (
	ErrListenAddr = errors.New("listen address must not be empty")
	ErrDBPath     = errors.New("db path must not be empty")
	ErrLokiURL    = errors.New("loki url must be an absolute URL with scheme and host")
	ErrGrafanaURL = errors.New("grafana url must be an absolute URL with scheme and host")
)

// envOr returns the value of the environment variable key, or def if unset.
func envOr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

// Load resolves settings from flags over env over canon defaults, validates
// the result, and returns a single validated *Config. On any validation
// failure it returns a nil *Config and a wrapped error — never a
// partially-initialized config.
func Load() (*Config, error) {
	// Read env before defining flags: flag reads its default at definition
	// time, so this is what makes the resolved env-or-default value become
	// each flag's default.
	listenDefault := envOr(envListen, defaultListen)
	lokiDefault := envOr(envLoki, defaultLoki)
	grafanaDefault := envOr(envGrafana, defaultGrafana)
	dbPathDefault := envOr(envDBPath, defaultDBPath)

	fs := flag.NewFlagSet("observe-write-proxy", flag.ContinueOnError)

	listenAddr := fs.String("listen", listenDefault, "address to listen on")
	lokiURL := fs.String("loki-url", lokiDefault, "base URL of the Loki instance")
	grafanaURL := fs.String("grafana-url", grafanaDefault, "base URL of the Grafana instance")
	dbPath := fs.String("db-path", dbPathDefault, "path to the SQLite token store")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, fmt.Errorf("config: parse flags: %w", err)
	}

	cfg := &Config{
		ListenAddr: *listenAddr,
		LokiURL:    *lokiURL,
		GrafanaURL: *grafanaURL,
		DBPath:     *dbPath,
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validate checks the resolved config values, returning a wrapped, named
// error on the first failure.
func validate(cfg *Config) error {
	if cfg.ListenAddr == "" {
		return fmt.Errorf("config: %w", ErrListenAddr)
	}
	if cfg.DBPath == "" {
		return fmt.Errorf("config: %w", ErrDBPath)
	}
	if !isValidBaseURL(cfg.LokiURL) {
		return fmt.Errorf("config: loki url %q: %w", cfg.LokiURL, ErrLokiURL)
	}
	if !isValidBaseURL(cfg.GrafanaURL) {
		return fmt.Errorf("config: grafana url %q: %w", cfg.GrafanaURL, ErrGrafanaURL)
	}
	return nil
}

// isValidBaseURL reports whether raw parses as an absolute URL carrying
// both a scheme and a host. url.Parse alone accepts scheme-less values like
// "localhost:3100" (Scheme="localhost", Opaque="3100", empty Host), so the
// explicit Host check is required to reject them.
func isValidBaseURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
}
