package opampserver

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	opampsrv "github.com/open-telemetry/opamp-go/server"
)

// OpAMPPath is the URL path OpAMP agents connect to, matching the path the
// OpAMP spec's examples and this project's collector manifests
// (deploy/k8s/collector-examples/) use by default.
const OpAMPPath = "/v1/opamp"

// maxPlainHTTPBodyBytes bounds the plain-HTTP OpAMP transport fallback (an
// agent posting a single AgentToServer protobuf message per request,
// instead of holding a WebSocket open). 1 MiB is generous for one such
// message. This limit does NOT apply to WebSocket connections: once
// upgraded, opamp-go reads directly off the hijacked net.Conn, bypassing
// http.Request.Body (and therefore http.MaxBytesReader) entirely. The
// underlying gorilla/websocket connection has no read-size limit configured
// by opamp-go v0.23.0 and the library exposes no way to set one from here --
// see docs/SECURITY.md for the accepted mitigation (pod memory limit).
const maxPlainHTTPBodyBytes = 1 << 20

// NewHTTPServer builds the *http.Server that serves the OpAMP endpoint.
//
// This deliberately does NOT use opamp-go's Server.Start(): that method
// builds its own internal http.Server with no ReadHeaderTimeout at all
// (verified against opamp-go v0.23.0's source), leaving the listener open
// to slow-header-style resource exhaustion. Using Server.Attach() instead
// gives us the handler function and ConnContext while letting us own the
// http.Server and set that timeout ourselves.
//
// ReadHeaderTimeout is set; ReadTimeout/WriteTimeout deliberately are NOT:
// those apply to a connection's entire lifetime, and would forcibly
// disconnect every agent's long-lived WebSocket once the timeout elapsed.
// ReadHeaderTimeout only bounds the time to read a request's headers,
// before any upgrade happens, which is exactly the slowloris mitigation
// needed here without harming established connections.
func NewHTTPServer(h *Handler, addr string, tlsConfig *tls.Config, log *slog.Logger) (*http.Server, error) {
	protocolServer := opampsrv.New(NewLogAdapter(log))
	handlerFunc, connContext, err := protocolServer.Attach(opampsrv.Settings{Callbacks: h.Callbacks()})
	if err != nil {
		return nil, fmt.Errorf("attach opamp protocol handler: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(OpAMPPath, func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxPlainHTTPBodyBytes)
		handlerFunc(w, r)
	})

	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		TLSConfig:         tlsConfig,
		ConnContext:       connContext,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}, nil
}
