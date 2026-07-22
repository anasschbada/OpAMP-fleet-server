package config

import "testing"

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func baseEnv() map[string]string {
	return map[string]string{
		"AGENT_AUTH_TOKENS_FILE": "/agent-tokens.txt",
		"API_AUTH_TOKENS_FILE":   "/api-tokens.txt",
	}
}

func TestLoad_MinimalValid(t *testing.T) {
	cfg, err := Load(envMap(baseEnv()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.OpAMPListenAddr != ":4320" {
		t.Errorf("default OpAMPListenAddr = %q", cfg.OpAMPListenAddr)
	}
	if cfg.APIListenAddr != ":8080" {
		t.Errorf("default APIListenAddr = %q", cfg.APIListenAddr)
	}
	if cfg.DataDir != "/data" {
		t.Errorf("default DataDir = %q", cfg.DataDir)
	}
}

func TestLoad_MissingAgentTokensFileRejected(t *testing.T) {
	env := baseEnv()
	delete(env, "AGENT_AUTH_TOKENS_FILE")
	if _, err := Load(envMap(env)); err == nil {
		t.Fatal("expected an error when AGENT_AUTH_TOKENS_FILE is unset")
	}
}

func TestLoad_MissingAPITokensFileRejected(t *testing.T) {
	env := baseEnv()
	delete(env, "API_AUTH_TOKENS_FILE")
	if _, err := Load(envMap(env)); err == nil {
		t.Fatal("expected an error when API_AUTH_TOKENS_FILE is unset")
	}
}

// This is the headline fix from the security review: a shared token pool
// between agents and the REST API let a compromised collector push config
// fleet-wide. Load must refuse to start rather than silently allow it.
func TestLoad_SharedTokenFileBetweenAgentAndAPIRejected(t *testing.T) {
	env := baseEnv()
	env["API_AUTH_TOKENS_FILE"] = env["AGENT_AUTH_TOKENS_FILE"]
	if _, err := Load(envMap(env)); err == nil {
		t.Fatal("expected an error when agent and API token files are the same path")
	}
}

func TestLoad_TLSMustBeBothOrNeither(t *testing.T) {
	env := baseEnv()
	env["TLS_CERT_FILE"] = "/tls.crt"
	// TLS_KEY_FILE deliberately left unset
	if _, err := Load(envMap(env)); err == nil {
		t.Fatal("expected an error when only TLS_CERT_FILE is set")
	}

	env["TLS_KEY_FILE"] = "/tls.key"
	if _, err := Load(envMap(env)); err != nil {
		t.Fatalf("unexpected error with both TLS files set: %v", err)
	}
}

func TestLoad_DisconnectedAfterMustExceedStaleAfter(t *testing.T) {
	env := baseEnv()
	env["STALE_AFTER"] = "90s"
	env["DISCONNECTED_AFTER"] = "30s"
	if _, err := Load(envMap(env)); err == nil {
		t.Fatal("expected an error when DISCONNECTED_AFTER <= STALE_AFTER")
	}
}

func TestLoad_InvalidDurationRejected(t *testing.T) {
	env := baseEnv()
	env["STALE_AFTER"] = "not-a-duration"
	if _, err := Load(envMap(env)); err == nil {
		t.Fatal("expected an error for a malformed STALE_AFTER")
	}
}
