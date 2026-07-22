// Command fleet-ui-server serves the built UI (ui/dist) as static files.
// It deliberately does not proxy to the OpAMP server's REST API: the
// browser calls that API directly (same-origin by default, or a
// configured base URL -- see ui/src/api/client.ts and README.md), so this
// process needs no egress network access at all and can run under a
// NetworkPolicy that allows ingress only, nothing outbound.
//
// A tiny Go static file server instead of nginx keeps the image's
// dependency surface identical to the main server (one language, one base
// image family, one set of CVEs to track) rather than adding an entirely
// different piece of software just to serve a handful of static files.
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal startup error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	addr := orDefault(os.Getenv("UI_LISTEN_ADDR"), ":8081")
	dir := orDefault(os.Getenv("STATIC_DIR"), "/www")

	// If the API is served from a different origin than this UI (e.g.
	// separate hostnames rather than one ingress with path-based routing),
	// set API_BASE_URL to that origin (e.g. "https://opamp-api.example.com").
	// Leave unset when UI and API share an origin (the common case): the
	// browser then calls same-origin relative paths, which is the default
	// in ui/src/api/client.ts.
	apiBaseURL := os.Getenv("API_BASE_URL")
	apiOrigin, err := originOf(apiBaseURL)
	if err != nil {
		return fmt.Errorf("API_BASE_URL: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	// Must be registered before the "/" static file handler below: Go's
	// ServeMux (1.22+) already prefers the more specific pattern, but being
	// explicit about ordering here avoids ever depending on that subtlety.
	mux.HandleFunc("GET /config.js", handleConfigJS(apiBaseURL))
	mux.Handle("/", withSecurityHeaders(apiOrigin, http.FileServer(http.Dir(dir))))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Info("ui server started", "addr", addr, "static_dir", dir, "api_base_url", apiBaseURL)
	return srv.ListenAndServe()
}

// handleConfigJS serves the small runtime-config script index.html loads
// before the app bundle, so the API base URL can be set per-deployment
// without rebuilding the UI image. Empty apiBaseURL still serves valid,
// well-formed JS (an empty string), which ui/src/api/client.ts treats as
// "same origin".
func handleConfigJS(apiBaseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store") // this value can change between deployments; never let the browser cache a stale one
		fmt.Fprintf(w, "window.__OPAMP_CONFIG__ = { apiBaseUrl: %q };\n", apiBaseURL)
	}
}

// originOf returns the scheme://host[:port] portion of rawURL, for use in
// a Content-Security-Policy connect-src directive. Returns "" (meaning
// "same-origin only") if rawURL is empty.
func originOf(rawURL string) (string, error) {
	if rawURL == "" {
		return "", nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("must be an absolute URL with scheme and host, got %q", rawURL)
	}
	return u.Scheme + "://" + u.Host, nil
}

// withSecurityHeaders applies a strict baseline appropriate for a static
// SPA: no inline script execution beyond what Vite's build emits as
// external files, no framing by other origins, and connect-src limited to
// 'self' plus whatever API origin API_BASE_URL configures.
func withSecurityHeaders(apiOrigin string, next http.Handler) http.Handler {
	connectSrc := "'self'"
	if apiOrigin != "" {
		connectSrc += " " + apiOrigin
	}
	csp := "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
		"connect-src " + connectSrc + "; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
