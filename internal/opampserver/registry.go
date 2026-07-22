package opampserver

import (
	"net"
	"sync"

	"github.com/open-telemetry/opamp-go/server/types"
)

// connEntry is what we track per live OpAMP connection: the connection
// itself (to push config), plus the two pieces of information the optional
// metrics scraper needs to reach that collector's self-telemetry endpoint.
//
// remoteIP is read from the actual TCP connection, never from anything the
// agent sends in-band -- this is deliberate: if we built a scrape URL from
// a self-reported hostname/IP string instead, a compromised or misconfigured
// collector could point the server at an arbitrary internal address
// (server-side request forgery). Using the already-authenticated
// connection's real peer address means the server only ever calls back an
// address it just verified is running an agent that holds a valid token.
type connEntry struct {
	conn        types.Connection
	remoteIP    string
	metricsPort uint16 // 0 = agent did not advertise a self-telemetry endpoint
}

// connRegistry tracks the live OpAMP connection for each connected agent, so
// a config push triggered by a REST API call (which has no Connection of its
// own) can find the right WebSocket to send on, and so the optional metrics
// scraper can find where to pull self-telemetry from. It is intentionally
// separate from the Store: connections are runtime-only and never persisted.
type connRegistry struct {
	mu    sync.RWMutex
	byUID map[string]connEntry
}

func newConnRegistry() *connRegistry {
	return &connRegistry{byUID: make(map[string]connEntry)}
}

// set registers (or refreshes) the live connection for an agent. It
// preserves whatever metricsPort was previously learned: AgentDescription
// (the only place metricsPort comes from) is omitted from most messages
// per the OpAMP spec ("may be omitted if unchanged since last message"), so
// this must NOT reset it to "unknown" on every heartbeat.
func (r *connRegistry) set(instanceUID string, conn types.Connection) {
	r.mu.Lock()
	defer r.mu.Unlock()
	prevPort := r.byUID[instanceUID].metricsPort
	r.byUID[instanceUID] = connEntry{
		conn:        conn,
		remoteIP:    remoteIPOf(conn),
		metricsPort: prevPort,
	}
}

// setMetricsPort updates the learned metrics port for an agent that has
// already called set. Called only when the current message actually
// carried an AgentDescription.
func (r *connRegistry) setMetricsPort(instanceUID string, port uint16) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.byUID[instanceUID]
	e.metricsPort = port
	r.byUID[instanceUID] = e
}

func remoteIPOf(conn types.Connection) string {
	nc := conn.Connection()
	if nc == nil || nc.RemoteAddr() == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(nc.RemoteAddr().String())
	if err != nil {
		return ""
	}
	return host
}

func (r *connRegistry) get(instanceUID string) (types.Connection, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.byUID[instanceUID]
	return e.conn, ok
}

// removeByConn deletes whichever entry currently holds this exact
// connection. OnConnectionClose only gives us the Connection, not the
// instance uid, so we scan the (small -- one entry per connected agent)
// map rather than maintaining a second reverse index. Matching by identity
// means a stale close event firing after a reconnect can never evict the
// new connection.
func (r *connRegistry) removeByConn(conn types.Connection) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for uid, e := range r.byUID {
		if e.conn == conn {
			delete(r.byUID, uid)
			return
		}
	}
}

// snapshot returns every currently connected agent's scrape target
// (instance uid, real remote IP, self-reported metrics port). Agents that
// didn't advertise a metrics port (metricsPort == 0) are skipped.
func (r *connRegistry) snapshot() []ScrapeTarget {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ScrapeTarget, 0, len(r.byUID))
	for uid, e := range r.byUID {
		if e.metricsPort == 0 || e.remoteIP == "" {
			continue
		}
		out = append(out, ScrapeTarget{InstanceUID: uid, RemoteIP: e.remoteIP, Port: e.metricsPort})
	}
	return out
}

// ScrapeTarget identifies where the optional metrics scraper (internal/metrics)
// can pull one connected agent's self-telemetry from.
type ScrapeTarget struct {
	InstanceUID string
	RemoteIP    string
	Port        uint16
}
