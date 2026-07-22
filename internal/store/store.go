// Package store defines the fleet registry: everything the server knows about
// connected OpenTelemetry Collector agents, their configuration history and
// health. All of this data originates from the OpAMP protocol itself (agents
// report it) -- nothing here is read from the Kubernetes API.
package store

import "time"

// ConnectionState is the liveness state the UI displays for an agent.
type ConnectionState string

const (
	StateConnected    ConnectionState = "connected"
	StateStale        ConnectionState = "stale"
	StateDisconnected ConnectionState = "disconnected"
)

// ConfigSyncState reflects whether the agent's effective config matches the
// last config the server pushed to it.
type ConfigSyncState string

const (
	ConfigSynced  ConfigSyncState = "synced"
	ConfigPending ConfigSyncState = "pending"
	ConfigDrifted ConfigSyncState = "drifted"
	ConfigFailed  ConfigSyncState = "failed"
)

// Agent is one OpenTelemetry Collector instance connected over OpAMP.
// Field values come exclusively from what the agent itself reports
// (AgentDescription attributes, EffectiveConfig, Health, RemoteConfigStatus)
// -- this works identically for any OTel Collector distribution (contrib,
// EDOT, a custom build, ...) as long as it runs the opamp extension.
type Agent struct {
	InstanceUID string // hex-encoded 16-byte OpAMP instance uid, primary key

	// Identifying attributes reported in AgentDescription. Populated from
	// well-known OpenTelemetry semantic convention keys when present:
	// service.name, service.namespace/k8s.namespace.name, service.version,
	// k8s.node.name, k8s.pod.name. Unrecognized keys are kept in Attributes.
	ServiceName  string
	Namespace    string
	Version      string
	NodeName     string
	PodName      string
	Attributes   map[string]string // full set of reported identifying + non-identifying attributes
	Capabilities uint64            // AgentCapabilities bitmask last reported

	ConnectionState ConnectionState
	LastSeen        time.Time
	StartTime       time.Time // from ComponentHealth.StartTimeUnixNano
	Healthy         bool
	LastError       string

	EffectiveConfigYAML  string
	ConfigSync           ConfigSyncState
	LastRemoteConfigHash []byte // hash of the config we last pushed, for RemoteConfigStatus comparison

	PushedBy     string // set by the REST API caller performing a config push, not part of OpAMP
	LastPushedAt time.Time
}

// ConfigPush is one historical config-push event, used by the "Historique"
// tab / diff view in the UI.
type ConfigPush struct {
	ID           string
	AgentUID     string
	Timestamp    time.Time
	ConfigYAML   string
	PushedBy     string
	Note         string
	Succeeded    bool // true once RemoteConfigStatus == APPLIED, false if FAILED
	ErrorMessage string
}

// Store is the persistence interface for the fleet registry. Implementations
// must be safe for concurrent use: OnMessage callbacks fire concurrently for
// different agent connections, and REST handlers read concurrently.
//
// Every mutating method returns an error instead of panicking: a transient
// disk error (full PVC, momentary I/O failure) must degrade a single
// request, never take down the whole process -- OpAMP callbacks in
// particular run inside the WebSocket read loop, and an unrecovered panic
// there would drop every other agent's connection too.
type Store interface {
	// UpsertAgent creates or updates an agent record. Callers pass a mutator
	// function so read-modify-write stays atomic under the store's own
	// transaction/lock.
	UpsertAgent(instanceUID string, mutate func(a *Agent)) (Agent, error)

	GetAgent(instanceUID string) (Agent, bool)
	ListAgents() ([]Agent, error)
	DeleteAgent(instanceUID string) error // called on prolonged disconnect cleanup, if ever needed

	// AppendConfigPush records a push attempt (called when the REST API
	// pushes a new remote config, and updated when the ack arrives).
	AppendConfigPush(push ConfigPush) error
	UpdateConfigPushResult(agentUID string, pushID string, succeeded bool, errMsg string) error
	ListConfigPushes(agentUID string) ([]ConfigPush, error)

	Close() error
}
