import type { Agent } from "../../types";

// Groups agents by the collector version they actually report, which
// catches version drift a namespace-level summary alone would hide. There
// is no image/tag field in the Agent DTO, only `version`, so that's the
// finest granularity available without a server-side change.
export function FleetComposition({ agents }: { agents: Agent[] }) {
  const rows = groupByVersion(agents);
  const majorityCount = rows.length > 0 ? rows[0].count : 0;

  return (
    <div className="panel" style={{ padding: 16 }}>
      <h3 style={{ margin: "0 0 4px", fontSize: 13.5, fontWeight: 600 }}>Versions des collecteurs</h3>
      <p style={{ margin: "0 0 12px", fontSize: 11.5, color: "var(--text-secondary)" }}>
        Version rapportée par chaque agent, groupée.
      </p>
      {rows.length === 0 ? (
        <div className="empty-state">Aucune donnée de version.</div>
      ) : (
        rows.map((r) => (
          <div className="version-row" key={r.version} title={r.version}>
            <span className="version-label mono">{r.version || "version inconnue"}</span>
            <div className="version-bar-track">
              <div className="version-bar-fill" style={{ width: `${(r.count / agents.length) * 100}%` }} />
            </div>
            <span className="version-count mono">
              {r.count}
              {rows.length > 1 && r.count < majorityCount && <span className="version-flag">minoritaire</span>}
            </span>
          </div>
        ))
      )}
    </div>
  );
}

function groupByVersion(agents: Agent[]): { version: string; count: number }[] {
  const counts = new Map<string, number>();
  for (const a of agents) {
    const v = a.version || "";
    counts.set(v, (counts.get(v) ?? 0) + 1);
  }
  return Array.from(counts, ([version, count]) => ({ version, count })).sort((a, b) => b.count - a.count);
}
