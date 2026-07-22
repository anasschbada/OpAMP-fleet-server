package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anasschbada/opamp-fleet-server/internal/auth"
	"github.com/anasschbada/opamp-fleet-server/internal/metrics"
	"github.com/anasschbada/opamp-fleet-server/internal/opampserver"
	"github.com/anasschbada/opamp-fleet-server/internal/store"
)

// testServer wires a full api.Handler against an in-memory store, exactly
// like production except for the backing Store implementation, so these
// tests exercise the real routing/auth/validation/rate-limiting chain.
type testServer struct {
	handler  http.Handler
	st       store.Store
	apiToken string
	agentTok string
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()

	writeTokenFile := func(name, token string) string {
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
			t.Fatalf("write token file: %v", err)
		}
		return path
	}
	agentTokenPath := writeTokenFile("agent-tokens.txt", "test-agent-token")
	apiTokenPath := writeTokenFile("api-tokens.txt", "test-api-token")

	agentTokens, err := auth.NewTokenVerifier(agentTokenPath)
	if err != nil {
		t.Fatalf("agent token verifier: %v", err)
	}
	apiTokens, err := auth.NewTokenVerifier(apiTokenPath)
	if err != nil {
		t.Fatalf("api token verifier: %v", err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	st := store.NewMemoryStore()
	opampHandler := opampserver.NewHandler(st, agentTokens, log)
	metricsStore := metrics.NewStore()

	handler := NewHandler(context.Background(), st, opampHandler, metricsStore, apiTokens, log)
	return &testServer{handler: handler, st: st, apiToken: "test-api-token", agentTok: "test-agent-token"}
}

func (ts *testServer) do(t *testing.T, method, path, token string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	ts.handler.ServeHTTP(rec, req)
	return rec
}

func TestAPI_HealthzReadyz_NoAuthRequired(t *testing.T) {
	ts := newTestServer(t)
	for _, path := range []string{"/healthz", "/readyz"} {
		rec := ts.do(t, http.MethodGet, path, "", nil)
		if rec.Code != http.StatusOK {
			t.Errorf("%s without auth = %d, want 200", path, rec.Code)
		}
	}
}

func TestAPI_RequiresAuth(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.do(t, http.MethodGet, "/api/v1/agents", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: got %d, want 401", rec.Code)
	}

	rec = ts.do(t, http.MethodGet, "/api/v1/agents", "wrong-token", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong token: got %d, want 401", rec.Code)
	}

	rec = ts.do(t, http.MethodGet, "/api/v1/agents", ts.apiToken, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("valid api token: got %d, want 200", rec.Code)
	}
}

// This is the headline finding from the security review: an agent token
// must never work against the REST API.
func TestAPI_AgentTokenRejectedOnAPI(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.do(t, http.MethodGet, "/api/v1/agents", ts.agentTok, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("agent-scoped token must not authenticate to the REST API: got %d, want 401", rec.Code)
	}
}

func TestAPI_RateLimitsFailedAuth(t *testing.T) {
	ts := newTestServer(t)
	var lastCode int
	for i := 0; i < 15; i++ {
		lastCode = ts.do(t, http.MethodGet, "/api/v1/agents", "bad-token", nil).Code
	}
	if lastCode != http.StatusTooManyRequests {
		t.Errorf("after repeated failures, got %d, want 429", lastCode)
	}
}

func TestAPI_PushConfig_UnknownAgent404(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.do(t, http.MethodPost, "/api/v1/agents/does-not-exist/config", ts.apiToken,
		[]byte(`{"configYaml":"a: b","note":""}`))
	if rec.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", rec.Code)
	}
}

func TestAPI_PushConfig_EmptyYAMLRejected(t *testing.T) {
	ts := newTestServer(t)
	if _, err := ts.st.UpsertAgent("agent-1", func(a *store.Agent) {}); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	rec := ts.do(t, http.MethodPost, "/api/v1/agents/agent-1/config", ts.apiToken,
		[]byte(`{"configYaml":"   ","note":""}`))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rec.Code)
	}
}

func TestAPI_PushConfig_MalformedYAMLRejected(t *testing.T) {
	ts := newTestServer(t)
	if _, err := ts.st.UpsertAgent("agent-1", func(a *store.Agent) {}); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	rec := ts.do(t, http.MethodPost, "/api/v1/agents/agent-1/config", ts.apiToken,
		[]byte(`{"configYaml":"a: [unterminated","note":""}`))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not valid YAML") {
		t.Errorf("expected a YAML-specific error message, got %q", rec.Body.String())
	}
}

func TestAPI_PushConfig_UnknownFieldRejected(t *testing.T) {
	ts := newTestServer(t)
	if _, err := ts.st.UpsertAgent("agent-1", func(a *store.Agent) {}); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	rec := ts.do(t, http.MethodPost, "/api/v1/agents/agent-1/config", ts.apiToken,
		[]byte(`{"configYaml":"a: b","note":"","unexpectedField":"x"}`))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400 for an unknown JSON field", rec.Code)
	}
}

func TestAPI_PushConfig_OversizedBodyRejected(t *testing.T) {
	ts := newTestServer(t)
	if _, err := ts.st.UpsertAgent("agent-1", func(a *store.Agent) {}); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	huge := bytes.Repeat([]byte("a"), 2<<20) // 2 MiB, over the 1 MiB cap
	body, _ := json.Marshal(map[string]string{"configYaml": string(huge)})
	rec := ts.do(t, http.MethodPost, "/api/v1/agents/agent-1/config", ts.apiToken, body)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400 for an oversized body", rec.Code)
	}
}

func TestAPI_PushConfig_ValidYAMLButAgentNotConnected(t *testing.T) {
	ts := newTestServer(t)
	if _, err := ts.st.UpsertAgent("agent-1", func(a *store.Agent) {}); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	rec := ts.do(t, http.MethodPost, "/api/v1/agents/agent-1/config", ts.apiToken,
		[]byte(`{"configYaml":"receivers:\n  otlp:\n","note":"test"}`))
	if rec.Code != http.StatusConflict {
		t.Errorf("got %d, want 409 (agent registered but no live OpAMP connection)", rec.Code)
	}
}

func TestAPI_GetAgent_SQLInjectionLikePathIsJustNotFound(t *testing.T) {
	ts := newTestServer(t)
	payloads := []string{
		"1' OR '1'='1",
		"'; DROP TABLE agents;--",
		"../../etc/passwd",
	}
	for _, p := range payloads {
		rec := ts.do(t, http.MethodGet, "/api/v1/agents/"+url.PathEscape(p), ts.apiToken, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("payload %q: got %d, want 404", p, rec.Code)
		}
	}
	// The store must still be intact and usable afterwards.
	rec := ts.do(t, http.MethodGet, "/api/v1/agents", ts.apiToken, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("store appears broken after injection attempts: %d", rec.Code)
	}
}

func TestAPI_NoCORSHeadersByDefault(t *testing.T) {
	ts := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+ts.apiToken)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	ts.handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("no CORS header should ever be set by default")
	}
}
