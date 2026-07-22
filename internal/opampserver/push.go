package opampserver

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/protobufs"

	"github.com/anasschbada/opamp-fleet-server/internal/store"
)

// ErrAgentNotConnected is returned by PushConfig when the target agent has
// no live OpAMP connection right now (it may reconnect later; the caller
// should surface this as "agent unreachable" rather than retry silently).
var ErrAgentNotConnected = errors.New("agent is not currently connected")

// PushConfig sends a new remote configuration to one connected agent over
// its existing OpAMP WebSocket connection. This is the ONLY way this server
// changes a collector's configuration -- there is no ConfigMap edit, no pod
// restart, and no Kubernetes API call involved, which is why the server
// needs no write access to the cluster at all (see docs/RBAC.md).
//
// The push is recorded immediately as a pending ConfigPush; it is marked
// succeeded/failed later, when the agent's next RemoteConfigStatus message
// arrives (handled in applyRemoteConfigStatus).
func (h *Handler) PushConfig(ctx context.Context, instanceUID, yamlBody, pushedBy, note string) (store.ConfigPush, error) {
	conn, ok := h.conns.get(instanceUID)
	if !ok {
		return store.ConfigPush{}, ErrAgentNotConnected
	}

	rawUID, err := uuid.Parse(instanceUID)
	if err != nil {
		return store.ConfigPush{}, fmt.Errorf("invalid instance uid %q: %w", instanceUID, err)
	}
	hash := configHash([]byte(yamlBody))

	push := store.ConfigPush{
		ID:         uuid.NewString(),
		AgentUID:   instanceUID,
		Timestamp:  time.Now().UTC(),
		ConfigYAML: yamlBody,
		PushedBy:   pushedBy,
		Note:       note,
		Succeeded:  false, // becomes true/false for real once the agent acks; "false" until then just means "not yet confirmed"
	}
	if err := h.store.AppendConfigPush(push); err != nil {
		return store.ConfigPush{}, fmt.Errorf("record config push: %w", err)
	}
	if _, err := h.store.UpsertAgent(instanceUID, func(a *store.Agent) {
		a.LastRemoteConfigHash = hash
		a.ConfigSync = store.ConfigPending
		a.PushedBy = pushedBy
		a.LastPushedAt = push.Timestamp
	}); err != nil {
		return store.ConfigPush{}, fmt.Errorf("update agent push state: %w", err)
	}

	msg := &protobufs.ServerToAgent{
		InstanceUid:  rawUID[:],
		Capabilities: serverCapabilities,
		RemoteConfig: &protobufs.AgentRemoteConfig{
			Config: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"": {Body: []byte(yamlBody), ContentType: "text/yaml"},
				},
			},
			ConfigHash: hash,
		},
	}
	if err := conn.Send(ctx, msg); err != nil {
		return push, fmt.Errorf("send remote config over opamp connection: %w", err)
	}
	return push, nil
}
