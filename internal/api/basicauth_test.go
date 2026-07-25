package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/anasschbada/opamp-fleet-server/internal/auth"
	"github.com/anasschbada/opamp-fleet-server/internal/metrics"
	"github.com/anasschbada/opamp-fleet-server/internal/opampserver"
	"github.com/anasschbada/opamp-fleet-server/internal/store"
)

// newTestServerWithBasicAuth is a variant of newTestServer (see
// agents_test.go) with HTTP Basic Auth also enabled, to exercise the
// additive bearer-or-basic path in withAuth without touching the
// bearer-only test fixture every other test in this package relies on.
func newTestServerWithBasicAuth(t *testing.T, username, password string) *testServer {
	t.Helper()

	writeFile := func(name, contents string) string {
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, []byte(contents+"\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return path
	}

	agentTokenPath := writeFile("agent-tokens.txt", "test-agent-token")
	apiTokenPath := writeFile("api-tokens.txt", "test-api-token")
	userPath := writeFile("basic-auth-username", username)
	passPath := writeFile("basic-auth-password", password)

	agentTokens, err := auth.NewTokenVerifier(agentTokenPath)
	if err != nil {
		t.Fatalf("agent token verifier: %v", err)
	}
	apiTokens, err := auth.NewTokenVerifier(apiTokenPath)
	if err != nil {
		t.Fatalf("api token verifier: %v", err)
	}
	basicAuth, err := auth.NewBasicAuthVerifier(userPath, passPath)
	if err != nil {
		t.Fatalf("basic auth verifier: %v", err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	st := store.NewMemoryStore()
	opampHandler := opampserver.NewHandler(st, agentTokens, log)
	metricsStore := metrics.NewStore()

	handler := NewHandler(context.Background(), st, opampHandler, metricsStore, apiTokens, basicAuth, log)
	return &testServer{handler: handler, st: st, apiToken: "test-api-token", agentTok: "test-agent-token"}
}

func TestAPI_BasicAuth_ValidCredentialGrantsAccess(t *testing.T) {
	ts := newTestServerWithBasicAuth(t, "admin", "s3cret-password")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	req.SetBasicAuth("admin", "s3cret-password")
	rec := httptest.NewRecorder()
	ts.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("valid basic auth credential: got %d, want 200", rec.Code)
	}
}

func TestAPI_BasicAuth_WrongPasswordRejected(t *testing.T) {
	ts := newTestServerWithBasicAuth(t, "admin", "s3cret-password")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	req.SetBasicAuth("admin", "wrong-password")
	rec := httptest.NewRecorder()
	ts.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong basic auth password: got %d, want 401", rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got == "" {
		t.Error("expected a WWW-Authenticate header when basic auth is enabled and the request is rejected")
	}
}

// The two login methods are additive, not exclusive: the existing bearer
// token pool must keep working unchanged once basic auth is turned on.
func TestAPI_BasicAuth_BearerTokenStillWorksWhenBasicAuthEnabled(t *testing.T) {
	ts := newTestServerWithBasicAuth(t, "admin", "s3cret-password")
	rec := ts.do(t, http.MethodGet, "/api/v1/agents", ts.apiToken, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("bearer token with basic auth also enabled: got %d, want 200", rec.Code)
	}
}

// Without basic auth configured at all (the default, see agents_test.go's
// newTestServer), a Basic Authorization header must be rejected exactly
// like any other invalid credential -- never silently ignored in a way
// that accidentally lets requests through unauthenticated.
func TestAPI_BasicAuth_RejectedWhenNotConfigured(t *testing.T) {
	ts := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	req.SetBasicAuth("admin", "anything")
	rec := httptest.NewRecorder()
	ts.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("basic auth header with the feature disabled: got %d, want 401", rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got != "" {
		t.Errorf("did not expect a WWW-Authenticate header when basic auth is disabled, got %q", got)
	}
}

// A collector's OpAMP agent token must never grant REST API access via
// either login method -- the core invariant this whole auth split exists
// for (see internal/config.Config's field comment).
func TestAPI_BasicAuth_AgentBearerTokenStillRejectedByAPI(t *testing.T) {
	ts := newTestServerWithBasicAuth(t, "admin", "s3cret-password")
	rec := ts.do(t, http.MethodGet, "/api/v1/agents", ts.agentTok, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("agent token against the API: got %d, want 401", rec.Code)
	}
}
