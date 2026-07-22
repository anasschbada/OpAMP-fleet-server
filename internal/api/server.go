// Package api implements the REST API the fleet UI talks to. It never
// touches the OpAMP protocol directly nor the Kubernetes API: it reads and
// writes through the Store, and triggers config pushes through
// opampserver.Handler, which owns the live agent connections.
package api

import (
	"log/slog"
	"net/http"

	"github.com/anasschbada/opamp-fleet-server/internal/auth"
	"github.com/anasschbada/opamp-fleet-server/internal/metrics"
	"github.com/anasschbada/opamp-fleet-server/internal/opampserver"
	"github.com/anasschbada/opamp-fleet-server/internal/store"
)

type Server struct {
	store   store.Store
	opamp   *opampserver.Handler
	metrics *metrics.Store // nil if metrics scraping is disabled
	log     *slog.Logger
}

// NewHandler builds the full HTTP handler for the REST API, with
// authentication, security headers, panic recovery and request logging
// applied around every route. /healthz and /readyz are intentionally
// unauthenticated so kubelet liveness/readiness probes don't need a
// credential -- they report process/store health only, never fleet data.
// metricsStore may be nil, in which case the metrics endpoint always
// responds with an empty snapshot (see handleGetAgentMetrics).
func NewHandler(st store.Store, opamp *opampserver.Handler, metricsStore *metrics.Store, tokens *auth.TokenVerifier, log *slog.Logger) http.Handler {
	s := &Server{store: st, opamp: opamp, metrics: metricsStore, log: log}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)

	api := http.NewServeMux()
	api.HandleFunc("GET /api/v1/agents", s.handleListAgents)
	api.HandleFunc("GET /api/v1/agents/{uid}", s.handleGetAgent)
	api.HandleFunc("GET /api/v1/agents/{uid}/config-pushes", s.handleListConfigPushes)
	api.HandleFunc("POST /api/v1/agents/{uid}/config", s.handlePushConfig)
	api.HandleFunc("GET /api/v1/agents/{uid}/metrics", s.handleGetAgentMetrics)
	api.HandleFunc("GET /api/v1/namespaces", s.handleListNamespaces)
	api.HandleFunc("GET /api/v1/component-catalog", s.handleComponentCatalog)
	mux.Handle("/api/v1/", withAuth(tokens, log, api))

	return withRecover(log, withSecurityHeaders(withLogging(log, mux)))
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// handleReadyz additionally confirms the store responds, so a Kubernetes
// readiness probe pulls a pod out of rotation if its database file becomes
// unreadable (e.g. PVC issue) instead of serving broken responses.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if _, err := s.store.ListAgents(); err != nil {
		writeError(w, s.log, http.StatusServiceUnavailable, "store unavailable", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
