package api

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/anasschbada/opamp-fleet-server/internal/auth"
	"github.com/anasschbada/opamp-fleet-server/internal/ratelimit"
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

// withAuth requires a valid bearer token on every request, from the
// API-scoped token set (never the OpAMP agent set -- see
// internal/config's field comment for why the two must stay separate).
// Health/readiness endpoints are registered outside this middleware (see
// server.go) so kubelet probes don't need a credential.
//
// Repeated failed attempts from one source IP are throttled (429) before
// the token is even checked, and every rejection is logged -- neither was
// true before this was added, which meant credential-guessing against the
// API left no trace and had no cost.
func withAuth(tokens *auth.TokenVerifier, limiter *ratelimit.FailureLimiter, log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)

		if !limiter.Allow(ip) {
			log.Warn("api request throttled: too many recent failed auth attempts", "remote_ip", ip, "path", r.URL.Path)
			writeError(w, log, http.StatusTooManyRequests, "too many failed attempts, try again later", nil)
			return
		}

		token := auth.BearerToken(r.Header.Get("Authorization"))
		if !tokens.Verify(token) {
			limiter.RecordFailure(ip)
			log.Warn("api request rejected: invalid or missing bearer token", "remote_ip", ip, "path", r.URL.Path)
			writeError(w, log, http.StatusUnauthorized, "unauthorized", nil)
			return
		}
		limiter.RecordSuccess(ip)
		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the request's direct TCP peer IP. Not
// X-Forwarded-For: trusting a client-supplied header here would let any
// caller spoof its rate-limit identity. If you place a reverse proxy in
// front of this API, extend this to trust X-Forwarded-For only from that
// proxy's known address.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
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
