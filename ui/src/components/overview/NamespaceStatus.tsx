import { useMemo, useState } from "react";
import type { Agent, NamespaceSummary } from "../../types";

type ViewMode = "table" | "cards";
type SortKey = "namespace" | "total" | "connected";
type Health = "critical" | "warning" | null;

// Table/cards toggle, cards by default -- cards read better at a glance for
// a handful of namespaces, the table earns its keep once you need to sort
// or compare many rows precisely. "Health" isn't a field on NamespaceSummary
// itself: it's derived here from the same connected/stale/disconnected
// counts the bars already show, plus an unhealthy count cross-referenced
// from the agents list (NamespaceSummary has no unhealthy field).
export function NamespaceStatus({
  namespaces,
  agents,
  onOpenNamespace,
}: {
  namespaces: NamespaceSummary[];
  agents: Agent[];
  onOpenNamespace: (ns: string) => void;
}) {
  const [viewMode, setViewMode] = useState<ViewMode>("cards");
  const [sortKey, setSortKey] = useState<SortKey>("total");
  const [sortDir, setSortDir] = useState<1 | -1>(-1);

  const unhealthyByNamespace = useMemo(() => {
    const map = new Map<string, number>();
    for (const a of agents) {
      if (a.healthy) continue;
      map.set(a.namespace, (map.get(a.namespace) ?? 0) + 1);
    }
    return map;
  }, [agents]);

  function health(ns: NamespaceSummary): Health {
    if ((unhealthyByNamespace.get(ns.namespace) ?? 0) > 0) return "critical";
    if (ns.stale > 0 || ns.disconnected > 0) return "warning";
    return null;
  }

  const sorted = useMemo(() => {
    const healthRank = (ns: NamespaceSummary) => (health(ns) === "critical" ? 0 : health(ns) === "warning" ? 1 : 2);
    return [...namespaces].sort((a, b) => {
      const byHealth = healthRank(a) - healthRank(b);
      if (byHealth !== 0) return byHealth;
      if (sortKey === "namespace") return a.namespace.localeCompare(b.namespace) * sortDir;
      return (a[sortKey] - b[sortKey]) * sortDir;
      // eslint-disable-next-line react-hooks/exhaustive-deps
    });
  }, [namespaces, sortKey, sortDir, unhealthyByNamespace]);

  function sortBy(key: SortKey) {
    if (key === sortKey) setSortDir((d) => (d * -1) as 1 | -1);
    else {
      setSortKey(key);
      setSortDir(key === "namespace" ? 1 : -1);
    }
  }

  return (
    <div>
      <div className="section-head">
        <div className="section-label">
          Statut des namespaces
          <span className="tag">
            {namespaces.length} namespace{namespaces.length > 1 ? "s" : ""}
          </span>
        </div>
        <div className="view-toggle">
          <button className={viewMode === "table" ? "active" : ""} onClick={() => setViewMode("table")}>
            Tableau
          </button>
          <button className={viewMode === "cards" ? "active" : ""} onClick={() => setViewMode("cards")}>
            Cartes
          </button>
        </div>
      </div>

      {namespaces.length === 0 ? (
        <div className="empty-state">Aucun agent connecté pour le moment.</div>
      ) : viewMode === "table" ? (
        <div className="panel">
          <table className="data-table">
            <thead>
              <tr>
                <SortableHeader label="Namespace" active={sortKey === "namespace"} dir={sortDir} onClick={() => sortBy("namespace")} />
                <th>Santé</th>
                <SortableHeader label="Agents" active={sortKey === "total"} dir={sortDir} onClick={() => sortBy("total")} />
                <SortableHeader label="Connectés" active={sortKey === "connected"} dir={sortDir} onClick={() => sortBy("connected")} />
                <th>Config synchronisée</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((ns) => (
                <tr key={ns.namespace} onClick={() => onOpenNamespace(ns.namespace)}>
                  <td className="mono">{ns.namespace}</td>
                  <td>
                    <HealthBadge health={health(ns)} />
                  </td>
                  <td className="mono">{ns.total}</td>
                  <td className="mono">{ns.connected}</td>
                  <td className="mono">
                    {ns.configSynced}/{ns.total}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="fleet-grid">
          {sorted.map((ns) => {
            const unhealthy = unhealthyByNamespace.get(ns.namespace) ?? 0;
            const sev = health(ns);
            return (
              <div
                key={ns.namespace}
                className={`panel fleet-card${sev ? " has-issue" : ""}`}
                onClick={() => onOpenNamespace(ns.namespace)}
              >
                <div className="fleet-card-top">
                  <span className="ns-name mono">{ns.namespace}</span>
                  {sev ? <span className="issue-flag">{sev === "critical" ? "Critique" : "Avertissement"}</span> : <span className="total">{ns.total}</span>}
                </div>
                {sev && <div className="total">{ns.total}</div>}
                <div style={{ fontSize: 11, color: "var(--text-secondary)" }}>
                  {ns.connected} connecté(s)
                  {unhealthy > 0 ? ` · ${unhealthy} en mauvaise santé` : ` · ${ns.configSynced}/${ns.total} synchronisées`}
                </div>
                <div className="stacked-bar">
                  {ns.connected > 0 && <div className="connected" style={{ flex: ns.connected }} />}
                  {ns.stale > 0 && <div className="stale" style={{ flex: ns.stale }} />}
                  {ns.disconnected > 0 && <div className="disconnected" style={{ flex: ns.disconnected }} />}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

function SortableHeader({ label, active, dir, onClick }: { label: string; active: boolean; dir: 1 | -1; onClick: () => void }) {
  return (
    <th
      onClick={onClick}
      role="button"
      tabIndex={0}
      aria-sort={active ? (dir === 1 ? "ascending" : "descending") : "none"}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onClick();
        }
      }}
    >
      {label}
      {active && <span style={{ marginLeft: 4, color: "var(--accent)" }}>{dir === 1 ? "▴" : "▾"}</span>}
    </th>
  );
}

function HealthBadge({ health }: { health: Health }) {
  if (health === "critical") return <span className="badge danger">Critique</span>;
  if (health === "warning") return <span className="badge warning">Avertissement</span>;
  return <span className="badge success">OK</span>;
}
