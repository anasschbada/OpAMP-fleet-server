import { useMemo, useState } from "react";
import type { Agent } from "../types";
import {
  connectionBadgeClass,
  connectionLabel,
  configSyncBadgeClass,
  configSyncLabel,
  relativeTime,
} from "../format";

// Consumes the fleet-wide agents list fetched once in App.tsx (see
// hooks/useFleetData.ts) instead of polling on its own -- this page and
// Overview/Alerts all read the same data so they can't drift apart.
export function AgentsList({
  agents,
  initialNamespace,
  onOpenAgent,
}: {
  agents: Agent[];
  initialNamespace: string | null;
  onOpenAgent: (uid: string) => void;
}) {
  const [search, setSearch] = useState("");
  const [namespace, setNamespace] = useState(initialNamespace ?? "");
  const [status, setStatus] = useState("");

  const namespaces = useMemo(() => Array.from(new Set(agents.map((a) => a.namespace).filter(Boolean))).sort(), [agents]);

  const filtered = agents.filter((a) => {
    if (namespace && a.namespace !== namespace) return false;
    if (status && a.connectionState !== status) return false;
    if (search) {
      const q = search.toLowerCase();
      if (!a.namespace.toLowerCase().includes(q) && !a.serviceName.toLowerCase().includes(q)) return false;
    }
    return true;
  });

  return (
    <div className="page">
      <div className="page-header">
        <h1>
          Agents <span className="badge success" style={{ marginLeft: 8 }}>LIVE</span>
        </h1>
        <p>{agents.length} collecteur(s) connu(s).</p>
      </div>

      <div className="filter-row">
        <input
          placeholder="Rechercher un namespace ou un service…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <select value={namespace} onChange={(e) => setNamespace(e.target.value)}>
          <option value="">Tous les namespaces</option>
          {namespaces.map((ns) => (
            <option key={ns} value={ns}>
              {ns}
            </option>
          ))}
        </select>
        <select value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">Tous les statuts</option>
          <option value="connected">Connecté</option>
          <option value="stale">Stale</option>
          <option value="disconnected">Déconnecté</option>
        </select>
      </div>

      <div className="panel">
        <table className="data-table">
          <thead>
            <tr>
              <th style={{ width: "15%" }}>Namespace</th>
              <th style={{ width: "25%" }}>Service</th>
              <th style={{ width: "13%" }}>Statut</th>
              <th style={{ width: "12%" }}>Vu</th>
              <th style={{ width: "10%" }}>Version</th>
              <th style={{ width: "17%" }}>Config</th>
              <th style={{ width: "8%" }} />
            </tr>
          </thead>
          <tbody>
            {filtered.map((a) => (
              <tr key={a.instanceUid} onClick={() => onOpenAgent(a.instanceUid)}>
                <td className="mono">{a.namespace || "—"}</td>
                <td>{a.serviceName || a.podName || a.instanceUid.slice(0, 8)}</td>
                <td>
                  <span className={`badge ${connectionBadgeClass[a.connectionState]}`}>
                    {connectionLabel[a.connectionState]}
                  </span>
                </td>
                <td className="mono">{relativeTime(a.lastSeen)}</td>
                <td className="mono">{a.version || "—"}</td>
                <td>
                  <span className={`badge ${configSyncBadgeClass[a.configSync]}`}>
                    {configSyncLabel[a.configSync]}
                  </span>
                </td>
                <td>›</td>
              </tr>
            ))}
          </tbody>
        </table>
        {filtered.length === 0 && <div className="empty-state">Aucun agent ne correspond à ces filtres.</div>}
      </div>
    </div>
  );
}
