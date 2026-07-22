package opampserver

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/protobufs"

	"github.com/anasschbada/opamp-fleet-server/internal/store"
)

func TestInstanceUIDString_RoundTrips(t *testing.T) {
	id := uuid.New()
	got, err := instanceUIDString(id[:])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != id.String() {
		t.Errorf("instanceUIDString() = %q, want %q", got, id.String())
	}
}

func TestInstanceUIDString_WrongLengthRejected(t *testing.T) {
	for _, n := range []int{0, 1, 15, 17, 32} {
		if _, err := instanceUIDString(make([]byte, n)); err == nil {
			t.Errorf("expected an error for a %d-byte instance uid", n)
		}
	}
}

func TestHashEqual(t *testing.T) {
	a := []byte{1, 2, 3}
	b := []byte{1, 2, 3}
	c := []byte{1, 2, 4}
	if !hashEqual(a, b) {
		t.Error("identical byte slices should be equal")
	}
	if hashEqual(a, c) {
		t.Error("differing byte slices should not be equal")
	}
	if hashEqual(a, []byte{1, 2}) {
		t.Error("different-length slices should never be equal")
	}
	// Two nil slices are both length 0, so the comparison loop never runs
	// and they compare equal -- pinned down explicitly since it's the edge
	// case that matters for a freshly created Agent with no push yet.
	if !hashEqual(nil, nil) {
		t.Error("two nil slices should compare equal")
	}
}

func TestConfigHash_DeterministicAndSensitive(t *testing.T) {
	h1 := configHash([]byte("receivers:\n  otlp:\n"))
	h2 := configHash([]byte("receivers:\n  otlp:\n"))
	h3 := configHash([]byte("receivers:\n  otlp2:\n"))
	if !bytes.Equal(h1, h2) {
		t.Error("identical input must hash identically")
	}
	if bytes.Equal(h1, h3) {
		t.Error("different input must hash differently")
	}
}

func TestJoinConfigMap(t *testing.T) {
	if got := joinConfigMap(nil); got != "" {
		t.Errorf("nil config map should join to empty string, got %q", got)
	}

	single := &protobufs.AgentConfigMap{
		ConfigMap: map[string]*protobufs.AgentConfigFile{"": {Body: []byte("a: b\n")}},
	}
	if got := joinConfigMap(single); got != "a: b\n" {
		t.Errorf("single unnamed entry should pass through verbatim, got %q", got)
	}

	multi := &protobufs.AgentConfigMap{
		ConfigMap: map[string]*protobufs.AgentConfigFile{
			"logs":    {Body: []byte("a: 1\n")},
			"metrics": {Body: []byte("b: 2\n")},
		},
	}
	got := joinConfigMap(multi)
	if !bytes.Contains([]byte(got), []byte("--- logs ---")) || !bytes.Contains([]byte(got), []byte("--- metrics ---")) {
		t.Errorf("multi-entry config map should label each section, got %q", got)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", "c"); got != "c" {
		t.Errorf("firstNonEmpty = %q, want %q", got, "c")
	}
	if got := firstNonEmpty("a", "b"); got != "a" {
		t.Errorf("firstNonEmpty = %q, want %q", got, "a")
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Errorf("firstNonEmpty of all-empty = %q, want empty", got)
	}
}

// applyRemoteConfigStatus is the state machine deciding whether a push
// succeeded, drifted, or failed -- worth pinning down precisely since it's
// what the UI's config-sync badge and history tab rely on.
func TestApplyRemoteConfigStatus(t *testing.T) {
	pushedHash := []byte{1, 2, 3}
	otherHash := []byte{9, 9, 9}

	cases := []struct {
		name           string
		agentHash      []byte
		status         *protobufs.RemoteConfigStatus
		wantSync       store.ConfigSyncState
		wantErrPresent bool
	}{
		{
			name:      "applied matching hash -> synced",
			agentHash: pushedHash,
			status: &protobufs.RemoteConfigStatus{
				LastRemoteConfigHash: pushedHash,
				Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
			},
			wantSync: store.ConfigSynced,
		},
		{
			name:      "applied with mismatched hash -> drifted",
			agentHash: pushedHash,
			status: &protobufs.RemoteConfigStatus{
				LastRemoteConfigHash: otherHash,
				Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
			},
			wantSync: store.ConfigDrifted,
		},
		{
			name:      "applying -> pending",
			agentHash: pushedHash,
			status: &protobufs.RemoteConfigStatus{
				Status: protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLYING,
			},
			wantSync: store.ConfigPending,
		},
		{
			name:      "failed -> failed, with error message",
			agentHash: pushedHash,
			status: &protobufs.RemoteConfigStatus{
				Status:       protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED,
				ErrorMessage: "boom",
			},
			wantSync:       store.ConfigFailed,
			wantErrPresent: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := &store.Agent{LastRemoteConfigHash: c.agentHash}
			applyRemoteConfigStatus(a, c.status)
			if a.ConfigSync != c.wantSync {
				t.Errorf("ConfigSync = %v, want %v", a.ConfigSync, c.wantSync)
			}
			if c.wantErrPresent && a.LastError == "" {
				t.Error("expected LastError to be set")
			}
		})
	}
}

// applyAgentToServer must not clobber previously known fields when a
// message omits AgentDescription/Health/EffectiveConfig, per the OpAMP
// spec's "may be omitted if unchanged" semantics.
func TestApplyAgentToServer_OmittedFieldsPreservePreviousState(t *testing.T) {
	a := &store.Agent{
		ServiceName:         "kept-name",
		EffectiveConfigYAML: "kept: config\n",
		Healthy:             true,
	}
	msg := &protobufs.AgentToServer{
		Capabilities: 42,
		// AgentDescription, Health, EffectiveConfig all nil/unset.
	}
	applyAgentToServer(a, msg, nil)

	if a.ServiceName != "kept-name" {
		t.Errorf("ServiceName was clobbered: %q", a.ServiceName)
	}
	if a.EffectiveConfigYAML != "kept: config\n" {
		t.Errorf("EffectiveConfigYAML was clobbered: %q", a.EffectiveConfigYAML)
	}
	if !a.Healthy {
		t.Error("Healthy was clobbered")
	}
	if a.ConnectionState != store.StateConnected {
		t.Error("every message, even a bare heartbeat, must mark the agent connected")
	}
	if a.Capabilities != 42 {
		t.Error("Capabilities should always be updated from the latest message")
	}
	if time.Since(a.LastSeen) > time.Second {
		t.Error("LastSeen should be set to approximately now")
	}
}
