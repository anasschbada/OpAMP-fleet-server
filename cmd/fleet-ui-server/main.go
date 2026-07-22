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
	"strings"
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
	mux.Handle("/", withRejectedControlChars(withSecurityHeaders(apiOrigin, http.FileServer(http.Dir(dir)))))

	srv := &http.Server{
		Addr:              addr,
		Handler:           withRecover(log, mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Info("ui server started", "addr", addr, "static_dir", dir, "api_base_url", apiBaseURL)
	return srv.ListenAndServe()
}

// withRecover turns a panic in any handler into a 500 without taking down
// the process. Go's net/http already recovers panics per-request
// internally, but silently (a bare stack trace to stderr, connection just
// dropped); this gives it a proper structured log line and a clean
// response instead.
func withRecover(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("panic recovered", "panic", rec, "path", r.URL.Path)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// withRejectedControlChars rejects any request path containing a null byte
// or other control character with a clean 400 before it ever reaches
// http.FileServer. Without this, a path like "/index.html%00.png" reaches
// os.Open with an embedded NUL, which fails with an error http.FileServer
// doesn't recognize as "not found" and surfaces as a bare 500 instead --
// not a path-traversal risk (no file content is exposed either way, and
// http.FileServer's own traversal protections are unaffected), just a
// noisy, uninformative status code for what's actually a malformed
// request.
func withRejectedControlChars(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.ContainsFunc(r.URL.Path, func(c rune) bool { return c < 0x20 || c == 0x7f }) {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
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
