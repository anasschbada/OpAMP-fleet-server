// Package opampserver implements the server side of the OpAMP protocol
// (https://github.com/open-telemetry/opamp-spec) using the open-telemetry/
// opamp-go library. It works with any OpenTelemetry Collector distribution
// that runs the "opamp" extension -- nothing here is specific to a
// particular collector build.
//
// Everything the fleet UI needs (agent identity, health, effective config,
// config-sync status) is derived from messages the agent itself sends. The
// handler never calls the Kubernetes API.
package opampserver

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server/types"

	"github.com/anasschbada/opamp-fleet-server/internal/auth"
	"github.com/anasschbada/opamp-fleet-server/internal/ratelimit"
	"github.com/anasschbada/opamp-fleet-server/internal/store"
)

// maxAuthFailures / authFailureWindow bound how many failed OpAMP
// connection attempts one source IP gets before being throttled outright,
// slowing down credential-guessing against the shared bearer tokens.
const (
	maxAuthFailures   = 10
	authFailureWindow = time.Minute
)

// serverCapabilities declares what this server offers: it can accept status
// reports, offer remote config, and accept the agent's reported effective
// config. It does NOT declare package/connection-settings offering -- this
// server manages collector configuration only, not binary upgrades.
const serverCapabilities = uint64(
	protobufs.ServerCapabilities_ServerCapabilities_AcceptsStatus |
		protobufs.ServerCapabilities_ServerCapabilities_OffersRemoteConfig |
		protobufs.ServerCapabilities_ServerCapabilities_AcceptsEffectiveConfig,
)

// Handler wires the OpAMP protocol callbacks to the fleet Store. One Handler
// instance is shared by every agent connection.
type Handler struct {
	store       store.Store
	agentTokens *auth.TokenVerifier // scoped to OpAMP connections only -- see internal/config's field comment for why this must differ from the REST API's token set
	conns       *connRegistry
	authLimiter *ratelimit.FailureLimiter
	log         *slog.Logger
}

// NewHandler wires a Handler. agentTokens must be the agent-scoped token
// set (internal/config.Config.AgentAuthTokensFile), never the REST API's.
func NewHandler(st store.Store, agentTokens *auth.TokenVerifier, log *slog.Logger) *Handler {
	return &Handler{
		store:       st,
		agentTokens: agentTokens,
		conns:       newConnRegistry(),
		authLimiter: ratelimit.NewFailureLimiter(maxAuthFailures, authFailureWindow),
		log:         log,
	}
}

// StartAuthLimiterCleanup evicts idle rate-limit tracking entries
// periodically so memory use doesn't grow with the number of distinct
// source IPs seen over the process lifetime. Call once at startup.
func (h *Handler) StartAuthLimiterCleanup(ctx context.Context, interval time.Duration) {
	h.authLimiter.StartCleanup(ctx, interval)
}

// Callbacks returns the opamp-go server.Settings.Callbacks wired to this
// Handler. Pass the result to server.Settings when starting the OpAMP
// listener (see cmd/opamp-server/main.go).
func (h *Handler) Callbacks() types.Callbacks {
	return types.Callbacks{
		OnConnecting: h.onConnecting,
	}
}

// ScrapeTargets returns the current set of connected agents that have
// advertised a self-telemetry metrics port, for the optional metrics
// scraper (internal/metrics) to poll. See registry.go's connEntry doc for
// why the address is always the live connection's real peer IP, never a
// self-reported host.
func (h *Handler) ScrapeTargets() []ScrapeTarget {
	return h.conns.snapshot()
}

// onConnecting authenticates the incoming connection before any OpAMP
// message is processed. OpAMP itself has no built-in authentication, so
// every collector must present a bearer token from the agent-scoped token
// set (see docs/RBAC.md and
// deploy/k8s/platform/04-secret-agent-tokens.example.yaml) -- never the
// REST API's token set, which must remain a separate pool (see
// internal/config's field comment for why).
func (h *Handler) onConnecting(req *http.Request) types.ConnectionResponse {
	remoteIP := clientIP(req)

	if !h.authLimiter.Allow(remoteIP) {
		h.log.Warn("opamp connection throttled: too many recent failed auth attempts", "remote_ip", remoteIP)
		return types.ConnectionResponse{Accept: false, HTTPStatusCode: http.StatusTooManyRequests}
	}

	token := auth.BearerToken(req.Header.Get("Authorization"))
	if !h.agentTokens.Verify(token) {
		h.authLimiter.RecordFailure(remoteIP)
		h.log.Warn("opamp connection rejected: invalid or missing bearer token", "remote_ip", remoteIP)
		return types.ConnectionResponse{Accept: false, HTTPStatusCode: http.StatusUnauthorized}
	}
	h.authLimiter.RecordSuccess(remoteIP)

	return types.ConnectionResponse{
		Accept: true,
		ConnectionCallbacks: types.ConnectionCallbacks{
			OnConnected:            h.onConnected,
			OnMessage:              h.onMessage,
			OnConnectionClose:      h.onConnectionClose,
			OnReadMessageError:     h.onReadMessageError,
			OnMessageResponseError: h.onMessageResponseError,
		},
	}
}

// clientIP extracts the request's source IP for rate-limiting purposes.
// This is the direct TCP peer, not an X-Forwarded-For header: OpAMP
// connections come straight from collector pods with no reverse proxy in
// front of this port in the deployment this project ships (see
// deploy/k8s/platform/09-networkpolicy.yaml), and trusting a
// client-supplied header here would let any agent spoof its rate-limit
// identity.
func clientIP(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return host
}

func (h *Handler) onConnected(ctx context.Context, conn types.Connection) {
	h.log.Debug("opamp connection established")
}

func (h *Handler) onConnectionClose(conn types.Connection) {
	// Drop it from the live-connection registry immediately so a config
	// push or metrics scrape doesn't try to use a dead socket; the Store's
	// ConnectionState still only flips to "disconnected" via the stale
	// sweeper, since a clean close and a network drop must be handled the
	// same way either way.
	h.conns.removeByConn(conn)
}

func (h *Handler) onReadMessageError(conn types.Connection, mt int, msgByte []byte, err error) {
	h.log.Warn("opamp read error", "error", err)
}

func (h *Handler) onMessageResponseError(conn types.Connection, message *protobufs.ServerToAgent, err error) {
	h.log.Warn("opamp send response error", "error", err)
}

// onMessage is the core protocol handler: every AgentToServer message
// (heartbeat, description update, health update, config-status update)
// flows through here. It updates the Store and returns whatever
// ServerToAgent response is appropriate.
func (h *Handler) onMessage(ctx context.Context, conn types.Connection, msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
	uid, err := instanceUIDString(msg.InstanceUid)
	if err != nil {
		h.log.Warn("opamp message with invalid instance_uid, dropping", "error", err)
		return &protobufs.ServerToAgent{InstanceUid: msg.InstanceUid, Capabilities: serverCapabilities}
	}

	h.conns.set(uid, conn)
	var attrs map[string]string
	if msg.AgentDescription != nil {
		attrs = flattenAttributes(msg.AgentDescription)
		h.conns.setMetricsPort(uid, metricsPortOf(attrs))
	}

	if _, err := h.store.UpsertAgent(uid, func(a *store.Agent) {
		applyAgentToServer(a, msg, attrs)
	}); err != nil {
		// A persistence failure must not drop the agent's connection or
		// crash the read loop; log and still answer the protocol so the
		// agent doesn't spin on reconnects. The next successful message
		// will retry the write.
		h.log.Error("failed to persist agent update", "instance_uid", uid, "error", err)
	}

	return &protobufs.ServerToAgent{
		InstanceUid:  msg.InstanceUid,
		Capabilities: serverCapabilities,
	}
}

// applyAgentToServer folds one AgentToServer message into the agent record.
// Every field is optional per the OpAMP spec ("may be omitted if unchanged
// since last message"), so each block only overwrites its own fields.
func applyAgentToServer(a *store.Agent, msg *protobufs.AgentToServer, attrs map[string]string) {
	now := time.Now().UTC()
	a.LastSeen = now
	a.ConnectionState = store.StateConnected
	a.Capabilities = msg.Capabilities

	if msg.AgentDescription != nil {
		a.Attributes = attrs
		a.ServiceName = firstNonEmpty(attrs[attrServiceName], a.ServiceName)
		// Prefer the Kubernetes-specific namespace attribute (set by the
		// k8sattributes processor) over the generic OTel service.namespace,
		// since it reflects where the pod actually runs.
		a.Namespace = firstNonEmpty(attrs[attrK8sNamespace], attrs[attrServiceNamespace], a.Namespace)
		a.Version = firstNonEmpty(attrs[attrServiceVersion], a.Version)
		a.NodeName = firstNonEmpty(attrs[attrK8sNodeName], a.NodeName)
		a.PodName = firstNonEmpty(attrs[attrK8sPodName], a.PodName)
	}

	if msg.Health != nil {
		a.Healthy = msg.Health.Healthy
		a.LastError = msg.Health.LastError
		// StartTimeUnixNano comes from the agent, not from us: guard the
		// uint64->int64 conversion against a value that would overflow
		// (whether from a malicious agent or a corrupt clock) instead of
		// silently wrapping into a bogus negative timestamp.
		if v := msg.Health.StartTimeUnixNano; v > 0 && v <= math.MaxInt64 {
			a.StartTime = time.Unix(0, int64(v)).UTC()
		}
	}

	if msg.EffectiveConfig != nil {
		a.EffectiveConfigYAML = joinConfigMap(msg.EffectiveConfig.ConfigMap)
	}

	if status := msg.RemoteConfigStatus; status != nil {
		applyRemoteConfigStatus(a, status)
	}
}

// applyRemoteConfigStatus updates ConfigSync based on the agent's report of
// how the last config we pushed was applied. Matching LastRemoteConfigHash
// against what we recorded when pushing (config.go's PushConfig) is how we
// tell "the agent applied OUR latest push" apart from "the agent applied
// some earlier push and hasn't seen the new one yet".
func applyRemoteConfigStatus(a *store.Agent, status *protobufs.RemoteConfigStatus) {
	hashMatches := len(a.LastRemoteConfigHash) > 0 && hashEqual(status.LastRemoteConfigHash, a.LastRemoteConfigHash)

	switch status.Status {
	case protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED:
		if hashMatches {
			a.ConfigSync = store.ConfigSynced
		} else {
			// Applied, but not the config we most recently pushed: either
			// another push raced ahead, or the agent's local config
			// changed independently of us.
			a.ConfigSync = store.ConfigDrifted
		}
	case protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLYING:
		a.ConfigSync = store.ConfigPending
	case protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED:
		a.ConfigSync = store.ConfigFailed
		a.LastError = firstNonEmpty(status.ErrorMessage, a.LastError)
	}
}

func hashEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// joinConfigMap flattens OpAMP's AgentConfigMap (a set of named config
// files/sections) into one YAML document for display. Collectors normally
// report a single unnamed entry; when there is more than one, each is
// separated with a YAML comment header naming it, so nothing is silently
// dropped for distributions that split config into sections.
func joinConfigMap(cm *protobufs.AgentConfigMap) string {
	if cm == nil || len(cm.ConfigMap) == 0 {
		return ""
	}
	if single, ok := cm.ConfigMap[""]; ok && len(cm.ConfigMap) == 1 {
		return string(single.Body)
	}
	out := ""
	for name, file := range cm.ConfigMap {
		if out != "" {
			out += "\n"
		}
		out += fmt.Sprintf("# --- %s ---\n%s", name, string(file.Body))
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// instanceUIDString renders the 16-byte OpAMP instance uid as a UUID
// string. The spec requires 16 bytes (UUIDv7 recommended); anything else is
// rejected rather than guessed at, since it's used as the Store's primary
// key.
func instanceUIDString(raw []byte) (string, error) {
	if len(raw) != 16 {
		return "", fmt.Errorf("instance_uid must be 16 bytes, got %d", len(raw))
	}
	id, err := uuid.FromBytes(raw)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// configHash computes the identifier the agent will echo back in
// RemoteConfigStatus.last_remote_config_hash once it applies this config.
func configHash(yamlBody []byte) []byte {
	sum := sha256.Sum256(yamlBody)
	return sum[:]
}
