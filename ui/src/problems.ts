// Derives "needs attention" entries purely from agents already fetched via
// listAgents() -- there is no alerts/incidents endpoint on the server, so
// this is the client reading signal that's already in the data (health,
// connection state, config sync) rather than synthesizing anything. Both
// the Overview page's capped feed and the full Alerts page call this same
// function, so the two views can never disagree with each other.
import type { Agent } from "./types";

export type Severity = "critical" | "warning";

export interface Problem {
  id: string;
  severity: Severity;
  title: string;
  detail: string;
  namespace: string;
  count: number;
}

export const severityLabel: Record<Severity, string> = {
  critical: "Critique",
  warning: "Avertissement",
};

export const severityBadgeClass: Record<Severity, string> = {
  critical: "danger",
  warning: "warning",
};

const severityRank: Record<Severity, number> = { critical: 0, warning: 1 };

export function deriveProblems(agents: Agent[]): Problem[] {
  const byNamespace = groupBy(agents, (a) => a.namespace || "(sans namespace)");
  const problems: Problem[] = [];

  for (const [namespace, nsAgents] of byNamespace) {
    const unhealthy = nsAgents.filter((a) => !a.healthy);
    if (unhealthy.length > 0) {
      problems.push({
        id: `unhealthy:${namespace}`,
        severity: "critical",
        title: `${unhealthy.length} agent${unhealthy.length > 1 ? "s" : ""} en mauvaise santé`,
        detail: unhealthy[0].lastError ? unhealthy[0].lastError : `Namespace ${namespace}`,
        namespace,
        count: unhealthy.length,
      });
    }

    const drifted = nsAgents.filter((a) => a.configSync === "drifted" || a.configSync === "failed");
    if (drifted.length > 0) {
      problems.push({
        id: `drift:${namespace}`,
        severity: "warning",
        title: "Dérive de configuration détectée",
        detail: `${drifted.length} agent${drifted.length > 1 ? "s" : ""} dans ${namespace} n'exécute${drifted.length > 1 ? "nt" : ""} pas la dernière configuration poussée`,
        namespace,
        count: drifted.length,
      });
    }

    const disconnected = nsAgents.filter((a) => a.connectionState === "disconnected");
    if (disconnected.length > 0) {
      problems.push({
        id: `disconnected:${namespace}`,
        severity: "warning",
        title: "Agents déconnectés",
        detail: `${disconnected.length} agent${disconnected.length > 1 ? "s" : ""} dans ${namespace} ne répond${disconnected.length > 1 ? "ent" : ""} plus`,
        namespace,
        count: disconnected.length,
      });
    }
  }

  return problems.sort((a, b) => {
    const bySeverity = severityRank[a.severity] - severityRank[b.severity];
    return bySeverity !== 0 ? bySeverity : b.count - a.count;
  });
}

function groupBy<T>(items: T[], key: (item: T) => string): Map<string, T[]> {
  const map = new Map<string, T[]>();
  for (const item of items) {
    const k = key(item);
    const list = map.get(k);
    if (list) list.push(item);
    else map.set(k, [item]);
  }
  return map;
}
