package opampserver

import (
	"context"
	"log/slog"
	"time"

	"github.com/anasschbada/opamp-fleet-server/internal/store"
)

// RunStaleSweeper periodically downgrades ConnectionState for agents that
// have stopped sending OpAMP messages. OpAMP has no server-initiated
// liveness probe -- an agent that dies mid-connection (killed pod, network
// partition) may never send a clean close -- so "connected" can only ever
// mean "sent a message within the last StaleAfter/DisconnectedAfter window".
//
// Blocks until ctx is cancelled; run it in its own goroutine.
func RunStaleSweeper(ctx context.Context, st store.Store, staleAfter, disconnectedAfter time.Duration, log *slog.Logger) {
	ticker := time.NewTicker(staleAfter / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweepOnce(st, staleAfter, disconnectedAfter, log)
		}
	}
}

func sweepOnce(st store.Store, staleAfter, disconnectedAfter time.Duration, log *slog.Logger) {
	agents, err := st.ListAgents()
	if err != nil {
		log.Error("stale sweeper: list agents failed", "error", err)
		return
	}

	now := time.Now().UTC()
	for _, a := range agents {
		age := now.Sub(a.LastSeen)
		want := store.StateConnected
		switch {
		case age >= disconnectedAfter:
			want = store.StateDisconnected
		case age >= staleAfter:
			want = store.StateStale
		}
		if want == a.ConnectionState {
			continue
		}
		if _, err := st.UpsertAgent(a.InstanceUID, func(agent *store.Agent) {
			agent.ConnectionState = want
		}); err != nil {
			log.Error("stale sweeper: update agent failed", "instance_uid", a.InstanceUID, "error", err)
		}
	}
}
