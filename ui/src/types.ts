// Mirrors internal/api/dto.go and internal/metrics/types.go on the server.
// Keep both sides in sync by hand -- there is no shared schema generation
// step in this project, and adding one for a handful of small DTOs would be
// more moving parts than it's worth.

export type ConnectionState = "connected" | "stale" | "disconnected";
export type ConfigSyncState = "synced" | "pending" | "drifted" | "failed";

export interface Agent {
  instanceUid: string;
  serviceName: string;
  namespace: string;
  version: string;
  nodeName: string;
  podName: string;
  attributes: Record<string, string>;
  connectionState: ConnectionState;
  lastSeen?: string;
  startTime?: string;
  healthy: boolean;
  lastError?: string;
  configSync: ConfigSyncState;
  pushedBy?: string;
  lastPushedAt?: string;
}

export interface AgentDetail extends Agent {
  effectiveConfigYaml: string;
}

export interface ConfigPush {
  id: string;
  timestamp: string;
  configYaml: string;
  pushedBy: string;
  note?: string;
  succeeded: boolean;
  errorMessage?: string;
}

export interface NamespaceSummary {
  namespace: string;
  total: number;
  connected: number;
  stale: number;
  disconnected: number;
  configSynced: number;
}

export interface MetricPoint {
  time: string;
  value: number;
}

export interface MetricsSnapshot {
  updatedAt: string;
  cpuSecondsPerSec: MetricPoint[];
  memoryRssBytes: MetricPoint[];
  receivedPointsRate: MetricPoint[];
  exportSuccessRatio: MetricPoint[];
}

export interface Component {
  id: string;
  label: string;
  signals: string[];
  yaml: string;
}

export interface ComponentCatalog {
  receivers: Component[];
  processors: Component[];
  connectors: Component[];
  exporters: Component[];
  extensions: Component[];
}
