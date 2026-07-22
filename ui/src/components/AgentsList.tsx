import { useEffect, useMemo, useState } from "react";
import { listAgents } from "../api/endpoints";
import type { Agent } from "../types";
import { ApiError } from "../api/client";
import {
  connectionBadgeClass,
  connectionLabel,
  configSyncBadgeClass,
  configSyncLabel,
  relativeTime,
} from "../format";

export function AgentsList({
  initialNamespace,
  onOpenAgent,
}: {
  initialNamespace: string | null;
  onOpenAgent: (uid: string) => void;
}) {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [namespace, setNamespace] = useState(initialNamespace ?? "");
  const [status, setStatus] = useState("");

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const a = await listAgents();
        if (!cancelled) setAgents(a);
      } catch (e) {
        if (!cancelled) setError(e instanceof ApiError ? e.message : "Erreur de chargement.");
      }
    }
    load();
    const id = setInterval(load, 10_000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);

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

      {error && <div className="error-banner">{error}</div>}

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
