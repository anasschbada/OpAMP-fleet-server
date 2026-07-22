import { api } from "./client";
import type {
  Agent,
  AgentDetail,
  ComponentCatalog,
  ConfigPush,
  MetricsSnapshot,
  NamespaceSummary,
} from "../types";

export const listAgents = () => api.get<Agent[]>("/api/v1/agents");
export const getAgent = (uid: string) => api.get<AgentDetail>(`/api/v1/agents/${encodeURIComponent(uid)}`);
export const listConfigPushes = (uid: string) =>
  api.get<ConfigPush[]>(`/api/v1/agents/${encodeURIComponent(uid)}/config-pushes`);
export const getAgentMetrics = (uid: string) =>
  api.get<MetricsSnapshot>(`/api/v1/agents/${encodeURIComponent(uid)}/metrics`);
export const listNamespaces = () => api.get<NamespaceSummary[]>("/api/v1/namespaces");
export const getComponentCatalog = () => api.get<ComponentCatalog>("/api/v1/component-catalog");

export const pushConfig = (uid: string, configYaml: string, note: string) =>
  api.post<ConfigPush>(`/api/v1/agents/${encodeURIComponent(uid)}/config`, { configYaml, note });
