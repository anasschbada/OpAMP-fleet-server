// Package metrics implements an OPTIONAL, best-effort collector of each
// agent's own self-telemetry (CPU, memory, pipeline throughput), so the
// fleet UI can show the "Métriques" tab and Overview traffic chart.
//
// This is deliberately separate from the OpAMP protocol: OpAMP carries
// identity, health and config, never arbitrary metric time series. Any OTel
// Collector already exposes its own operational metrics in Prometheus text
// format (the "telemetry.metrics" self-monitoring endpoint, conventionally
// port 8888) -- this package just polls that endpoint for collectors that
// choose to advertise it.
//
// Nothing here is required for fleet management to work: an agent that
// never advertises a metrics port simply shows no metrics tab data, with
// health/config/status still fully functional over OpAMP alone.
package metrics

import "time"

// Point is one sample in a metric's short history, used to draw sparklines.
type Point struct {
	Time  time.Time `json:"time"`
	Value float64   `json:"value"`
}

// maxHistoryPoints caps how many samples we keep per agent per metric.
// At the default 15s scrape interval this is ~15 minutes of history --
// enough for the UI's sparklines, small enough to keep memory use trivial
// even for a fleet of thousands of agents.
const maxHistoryPoints = 60

// Snapshot is the latest known metrics for one agent.
type Snapshot struct {
	UpdatedAt time.Time `json:"updatedAt"`

	CPUSecondsPerSec   []Point `json:"cpuSecondsPerSec"`   // rate of otelcol_process_cpu_seconds_total
	MemoryRSSBytes     []Point `json:"memoryRssBytes"`     // otelcol_process_uptime / memory gauge, as reported
	ReceivedPointsRate []Point `json:"receivedPointsRate"` // rate of otelcol_receiver_accepted_*_total, summed across signals
	ExportSuccessRatio []Point `json:"exportSuccessRatio"` // sent / (sent + failed), across all exporters, 0..1
}

func appendCapped(series []Point, p Point) []Point {
	series = append(series, p)
	if len(series) > maxHistoryPoints {
		series = series[len(series)-maxHistoryPoints:]
	}
	return series
}
