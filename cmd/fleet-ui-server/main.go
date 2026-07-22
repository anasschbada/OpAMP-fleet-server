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
	"log/slog"
	"net/http"
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
	// set API_ORIGIN to that origin so the CSP's connect-src allows the
	// browser's fetch() calls to reach it. Leave unset when UI and API
	// share an origin (the common case, and the default in
	// ui/src/api/client.ts).
	apiOrigin := os.Getenv("API_ORIGIN")

	fileServer := http.FileServer(http.Dir(dir))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.Handle("/", withSecurityHeaders(apiOrigin, fileServer))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Info("ui server started", "addr", addr, "static_dir", dir)
	return srv.ListenAndServe()
}

// withSecurityHeaders applies a strict baseline appropriate for a static
// SPA: no inline script execution beyond what Vite's build emits as
// external files, no framing by other origins, and connect-src left to
// 'self' plus whatever API origin the deployment configures via
// window.__OPAMP_CONFIG__ (see index.html) -- adjust connect-src below if
// the API is served from a different origin than the UI.
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
