// Package config loads and validates server settings from environment
// variables. Kubernetes-native deployments configure containers this way
// (env vars from a ConfigMap/Secret), so there is no separate config file
// format to parse, template, or keep in sync with the manifests.
package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds every runtime setting the server needs. Zero values are never
// silently used for security-relevant fields (auth tokens, TLS): Load
// returns an error instead, so misconfiguration fails startup rather than
// running unprotected.
type Config struct {
	// OpAMPListenAddr is where the OpAMP WebSocket/HTTP endpoint listens,
	// e.g. ":4320". Collectors connect here.
	OpAMPListenAddr string
	// APIListenAddr is where the REST API (consumed by the UI) listens,
	// e.g. ":8080".
	APIListenAddr string

	// TLSCertFile / TLSKeyFile enable TLS on both listeners when set. In a
	// secured/airgapped cluster this is expected to be a cert issued by the
	// organization's internal CA (e.g. via cert-manager with a private
	// ClusterIssuer already present in the cluster -- this server does not
	// provision one itself, see docs/RBAC.md).
	TLSCertFile string
	TLSKeyFile  string

	// AgentAuthTokensFile and APIAuthTokensFile point at two SEPARATE sets
	// of bearer tokens (one per line each): one accepted from connecting
	// OpAMP agents, one accepted from REST API callers (the UI, or a human
	// operator). Keeping them separate matters -- a collector only ever
	// needs to prove it may open an OpAMP connection; it must NOT also be
	// able to call the REST API and push configuration to every other
	// agent in the fleet. If a single compromised collector pod held a
	// token valid for both, that compromise would escalate into a
	// fleet-wide config-injection primitive. Each file supports more than
	// one token so it can be rotated without downtime (add the new token,
	// roll the consumers, remove the old one); both are re-read
	// periodically without a pod restart (see
	// internal/auth.TokenVerifier.StartAutoReload).
	AgentAuthTokensFile string
	APIAuthTokensFile   string

	// DataDir is where the SQLite database file lives. Must be a writable,
	// persistent path (a PersistentVolumeClaim mount in Kubernetes).
	DataDir string

	// StaleAfter / DisconnectedAfter classify an agent's ConnectionState
	// based on time since its last OpAMP message, since OpAMP itself has no
	// server-initiated liveness probe.
	StaleAfter        time.Duration
	DisconnectedAfter time.Duration

	LogLevel string
}

// Load reads configuration from the process environment and validates it.
func Load(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}

	cfg := Config{
		OpAMPListenAddr:     orDefault(getenv("OPAMP_LISTEN_ADDR"), ":4320"),
		APIListenAddr:       orDefault(getenv("API_LISTEN_ADDR"), ":8080"),
		TLSCertFile:         getenv("TLS_CERT_FILE"),
		TLSKeyFile:          getenv("TLS_KEY_FILE"),
		AgentAuthTokensFile: getenv("AGENT_AUTH_TOKENS_FILE"),
		APIAuthTokensFile:   getenv("API_AUTH_TOKENS_FILE"),
		DataDir:             orDefault(getenv("DATA_DIR"), "/data"),
		LogLevel:            orDefault(getenv("LOG_LEVEL"), "info"),
	}

	staleAfter, err := parseDurationOrDefault(getenv("STALE_AFTER"), 30*time.Second)
	if err != nil {
		return Config{}, fmt.Errorf("STALE_AFTER: %w", err)
	}
	cfg.StaleAfter = staleAfter

	disconnectedAfter, err := parseDurationOrDefault(getenv("DISCONNECTED_AFTER"), 90*time.Second)
	if err != nil {
		return Config{}, fmt.Errorf("DISCONNECTED_AFTER: %w", err)
	}
	cfg.DisconnectedAfter = disconnectedAfter

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.AgentAuthTokensFile == "" {
		return fmt.Errorf("AGENT_AUTH_TOKENS_FILE must be set: refusing to start without OpAMP agent authentication configured")
	}
	if c.APIAuthTokensFile == "" {
		return fmt.Errorf("API_AUTH_TOKENS_FILE must be set: refusing to start without REST API authentication configured")
	}
	if c.AgentAuthTokensFile == c.APIAuthTokensFile {
		return fmt.Errorf("AGENT_AUTH_TOKENS_FILE and API_AUTH_TOKENS_FILE must not be the same file: " +
			"a compromised collector must not also hold a valid REST API token (see the field comment)")
	}
	if (c.TLSCertFile == "") != (c.TLSKeyFile == "") {
		return fmt.Errorf("TLS_CERT_FILE and TLS_KEY_FILE must both be set, or both left empty")
	}
	if c.StaleAfter <= 0 || c.DisconnectedAfter <= c.StaleAfter {
		return fmt.Errorf("DISCONNECTED_AFTER must be greater than STALE_AFTER, both must be positive")
	}
	return nil
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func parseDurationOrDefault(v string, def time.Duration) (time.Duration, error) {
	if v == "" {
		return def, nil
	}
	return time.ParseDuration(v)
}
