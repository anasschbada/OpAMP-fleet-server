package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/anasschbada/opamp-fleet-server/internal/auth"
)

// withRecover turns a panic in any handler into a 500 response instead of
// crashing the whole process -- one bad request must never take down every
// other in-flight request.
func withRecover(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("panic recovered", "panic", rec, "path", r.URL.Path)
				writeError(w, log, http.StatusInternalServerError, "internal server error", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// withSecurityHeaders sets response headers appropriate for a JSON API that
// may be reachable from a browser (the fleet UI). Even though this API
// serves no HTML itself, a restrictive CSP/frame policy costs nothing and
// blocks this origin from being embedded or sniffed as something else.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// withAuth requires a valid bearer token on every request. Health/readiness
// endpoints are registered outside this middleware (see server.go) so
// kubelet probes don't need a credential.
func withAuth(tokens *auth.TokenVerifier, log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := auth.BearerToken(r.Header.Get("Authorization"))
		if !tokens.Verify(token) {
			writeError(w, log, http.StatusUnauthorized, "unauthorized", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withLogging logs one line per request: method, path, status, duration.
// No request/response bodies are logged, so config YAML and tokens never
// end up in log output.
func withLogging(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Info("http request",
			"method", r.Method, "path", r.URL.Path,
			"status", sw.status, "duration_ms", time.Since(start).Milliseconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
