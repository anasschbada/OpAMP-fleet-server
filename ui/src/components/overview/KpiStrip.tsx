import type { Agent } from "../../types";

// Self-contained: derives its four counters straight from the agents list
// it's given, no props beyond that and no fetching of its own.
export function KpiStrip({ agents }: { agents: Agent[] }) {
  const total = agents.length;
  const connected = agents.filter((a) => a.connectionState === "connected").length;
  const unhealthy = agents.filter((a) => !a.healthy).length;
  const drift = agents.filter((a) => a.configSync === "drifted" || a.configSync === "failed").length;

  return (
    <div className="kpi-row">
      <Kpi label="Total agents" value={total} />
      <Kpi label="Connectés" value={connected} />
      <Kpi label="En mauvaise santé" value={unhealthy} tone={unhealthy > 0 ? "danger" : undefined} />
      <Kpi label="Dérive de config" value={drift} tone={drift > 0 ? "warning" : undefined} />
    </div>
  );
}

function Kpi({ label, value, tone }: { label: string; value: number; tone?: "danger" | "warning" }) {
  return (
    <div className="panel kpi-card">
      <div className="label">{label}</div>
      <div className="value mono" style={tone ? { color: `var(--${tone})` } : undefined}>
        {value}
      </div>
    </div>
  );
}
