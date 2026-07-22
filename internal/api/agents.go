package api

import (
	"errors"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/anasschbada/opamp-fleet-server/internal/metrics"
	"github.com/anasschbada/opamp-fleet-server/internal/opampserver"
)

// emptyMetricsSnapshot is returned whenever an agent has no metrics data
// yet, so the JSON shape returned by the endpoint is always the same
// (empty arrays, not a null/absent snapshot) regardless of why.
var emptyMetricsSnapshot = metrics.Snapshot{}

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := s.store.ListAgents()
	if err != nil {
		writeError(w, s.log, http.StatusInternalServerError, "failed to list agents", err)
		return
	}
	dtos := make([]agentDTO, 0, len(agents))
	for _, a := range agents {
		dtos = append(dtos, toAgentDTO(a))
	}
	writeJSON(w, http.StatusOK, dtos)
}

func (s *Server) handleListNamespaces(w http.ResponseWriter, r *http.Request) {
	agents, err := s.store.ListAgents()
	if err != nil {
		writeError(w, s.log, http.StatusInternalServerError, "failed to list agents", err)
		return
	}
	writeJSON(w, http.StatusOK, summarizeNamespaces(agents))
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	uid := r.PathValue("uid")
	agent, ok := s.store.GetAgent(uid)
	if !ok {
		writeError(w, s.log, http.StatusNotFound, "agent not found", nil)
		return
	}
	writeJSON(w, http.StatusOK, toAgentDetailDTO(agent))
}

func (s *Server) handleListConfigPushes(w http.ResponseWriter, r *http.Request) {
	uid := r.PathValue("uid")
	if _, ok := s.store.GetAgent(uid); !ok {
		writeError(w, s.log, http.StatusNotFound, "agent not found", nil)
		return
	}
	pushes, err := s.store.ListConfigPushes(uid)
	if err != nil {
		writeError(w, s.log, http.StatusInternalServerError, "failed to list config pushes", err)
		return
	}
	dtos := make([]configPushDTO, 0, len(pushes))
	for _, p := range pushes {
		dtos = append(dtos, toConfigPushDTO(p))
	}
	writeJSON(w, http.StatusOK, dtos)
}

type pushConfigRequest struct {
	ConfigYAML string `json:"configYaml"`
	Note       string `json:"note"`
}

// handlePushConfig sends a new remote configuration to one agent over its
// live OpAMP connection (see opampserver.PushConfig). The agent must be
// currently connected -- there is no queued/offline delivery, matching the
// OpAMP protocol's own semantics.
func (s *Server) handlePushConfig(w http.ResponseWriter, r *http.Request) {
	uid := r.PathValue("uid")
	if _, ok := s.store.GetAgent(uid); !ok {
		writeError(w, s.log, http.StatusNotFound, "agent not found", nil)
		return
	}

	var req pushConfigRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, s.log, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if strings.TrimSpace(req.ConfigYAML) == "" {
		writeError(w, s.log, http.StatusBadRequest, "configYaml must not be empty", nil)
		return
	}
	// Reject syntactically invalid YAML before it ever reaches the
	// collector: a bad push should fail fast in the API response, not
	// surface only as a cryptic RemoteConfigStatus=FAILED minutes later.
	var probe any
	if err := yaml.Unmarshal([]byte(req.ConfigYAML), &probe); err != nil {
		writeError(w, s.log, http.StatusBadRequest, "configYaml is not valid YAML: "+err.Error(), nil)
		return
	}

	pushedBy := resolvePushedBy(r)
	push, err := s.opamp.PushConfig(r.Context(), uid, req.ConfigYAML, pushedBy, req.Note)
	if err != nil {
		if errors.Is(err, opampserver.ErrAgentNotConnected) {
			writeError(w, s.log, http.StatusConflict, "agent is not currently connected", err)
			return
		}
		writeError(w, s.log, http.StatusInternalServerError, "failed to push configuration", err)
		return
	}
	writeJSON(w, http.StatusAccepted, toConfigPushDTO(push))
}

// handleGetAgentMetrics returns the agent's latest self-telemetry snapshot,
// if metrics scraping is enabled and this agent has advertised an endpoint
// for it (see internal/metrics). Absence is not an error: it just means
// this section of the UI has nothing to show for this agent yet.
func (s *Server) handleGetAgentMetrics(w http.ResponseWriter, r *http.Request) {
	uid := r.PathValue("uid")
	if _, ok := s.store.GetAgent(uid); !ok {
		writeError(w, s.log, http.StatusNotFound, "agent not found", nil)
		return
	}
	if s.metrics == nil {
		writeJSON(w, http.StatusOK, emptyMetricsSnapshot)
		return
	}
	snap, ok := s.metrics.Get(uid)
	if !ok {
		writeJSON(w, http.StatusOK, emptyMetricsSnapshot)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleComponentCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, defaultCatalog)
}

// resolvePushedBy identifies the human who requested a config push, for the
// audit trail shown in the "Historique" tab. It trusts an authenticating
// reverse proxy in front of this API (e.g. oauth2-proxy, or an ingress
// performing OIDC auth) to set X-Forwarded-User/X-Forwarded-Email; without
// one, there is no verified identity system here, and the value falls back
// to a generic label. Deployments that need per-user accountability MUST
// place such a proxy in front of this API -- this server does not implement
// its own login system.
func resolvePushedBy(r *http.Request) string {
	if u := r.Header.Get("X-Forwarded-User"); u != "" {
		return u
	}
	if u := r.Header.Get("X-Forwarded-Email"); u != "" {
		return u
	}
	return "operator"
}
