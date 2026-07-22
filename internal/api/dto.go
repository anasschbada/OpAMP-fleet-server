package api

import (
	"time"

	"github.com/anasschbada/opamp-fleet-server/internal/store"
)

// agentDTO is the JSON shape returned to the UI. It's kept separate from
// store.Agent so internal storage fields (or future additions) don't leak
// into the API contract without an explicit decision here.
type agentDTO struct {
	InstanceUID     string            `json:"instanceUid"`
	ServiceName     string            `json:"serviceName"`
	Namespace       string            `json:"namespace"`
	Version         string            `json:"version"`
	NodeName        string            `json:"nodeName"`
	PodName         string            `json:"podName"`
	Attributes      map[string]string `json:"attributes"`
	ConnectionState string            `json:"connectionState"`
	LastSeen        *time.Time        `json:"lastSeen,omitempty"`
	StartTime       *time.Time        `json:"startTime,omitempty"`
	Healthy         bool              `json:"healthy"`
	LastError       string            `json:"lastError,omitempty"`
	ConfigSync      string            `json:"configSync"`
	PushedBy        string            `json:"pushedBy,omitempty"`
	LastPushedAt    *time.Time        `json:"lastPushedAt,omitempty"`
}

// agentDetailDTO extends agentDTO with the effective config, which is
// omitted from the list endpoint to keep GET /agents lightweight (config
// bodies can be several KB each).
type agentDetailDTO struct {
	agentDTO
	EffectiveConfigYAML string `json:"effectiveConfigYaml"`
}

func toAgentDTO(a store.Agent) agentDTO {
	return agentDTO{
		InstanceUID:     a.InstanceUID,
		ServiceName:     a.ServiceName,
		Namespace:       a.Namespace,
		Version:         a.Version,
		NodeName:        a.NodeName,
		PodName:         a.PodName,
		Attributes:      a.Attributes,
		ConnectionState: string(a.ConnectionState),
		LastSeen:        timeOrNil(a.LastSeen),
		StartTime:       timeOrNil(a.StartTime),
		Healthy:         a.Healthy,
		LastError:       a.LastError,
		ConfigSync:      string(a.ConfigSync),
		PushedBy:        a.PushedBy,
		LastPushedAt:    timeOrNil(a.LastPushedAt),
	}
}

func toAgentDetailDTO(a store.Agent) agentDetailDTO {
	return agentDetailDTO{agentDTO: toAgentDTO(a), EffectiveConfigYAML: a.EffectiveConfigYAML}
}

func timeOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

type configPushDTO struct {
	ID           string    `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	ConfigYAML   string    `json:"configYaml"`
	PushedBy     string    `json:"pushedBy"`
	Note         string    `json:"note,omitempty"`
	Succeeded    bool      `json:"succeeded"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
}

func toConfigPushDTO(p store.ConfigPush) configPushDTO {
	return configPushDTO{
		ID:           p.ID,
		Timestamp:    p.Timestamp,
		ConfigYAML:   p.ConfigYAML,
		PushedBy:     p.PushedBy,
		Note:         p.Note,
		Succeeded:    p.Succeeded,
		ErrorMessage: p.ErrorMessage,
	}
}

// namespaceSummaryDTO backs the Overview page's per-namespace fleet cards.
type namespaceSummaryDTO struct {
	Namespace    string `json:"namespace"`
	Total        int    `json:"total"`
	Connected    int    `json:"connected"`
	Stale        int    `json:"stale"`
	Disconnected int    `json:"disconnected"`
	ConfigSynced int    `json:"configSynced"`
}

func summarizeNamespaces(agents []store.Agent) []namespaceSummaryDTO {
	byNS := map[string]*namespaceSummaryDTO{}
	var order []string
	for _, a := range agents {
		ns := a.Namespace
		if ns == "" {
			ns = "(sans namespace)"
		}
		s, ok := byNS[ns]
		if !ok {
			s = &namespaceSummaryDTO{Namespace: ns}
			byNS[ns] = s
			order = append(order, ns)
		}
		s.Total++
		switch a.ConnectionState {
		case store.StateConnected:
			s.Connected++
		case store.StateStale:
			s.Stale++
		case store.StateDisconnected:
			s.Disconnected++
		}
		if a.ConfigSync == store.ConfigSynced {
			s.ConfigSynced++
		}
	}
	out := make([]namespaceSummaryDTO, 0, len(order))
	for _, ns := range order {
		out = append(out, *byNS[ns])
	}
	return out
}
