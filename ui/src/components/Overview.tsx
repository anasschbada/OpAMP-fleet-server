import { useEffect, useState } from "react";
import { listAgents, listNamespaces } from "../api/endpoints";
import type { Agent, NamespaceSummary } from "../types";
import { ApiError } from "../api/client";

export function Overview({
  onOpenNamespace,
  onOpenAgent,
}: {
  onOpenNamespace: (ns: string) => void;
  onOpenAgent: (uid: string) => void;
}) {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [namespaces, setNamespaces] = useState<NamespaceSummary[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const [a, ns] = await Promise.all([listAgents(), listNamespaces()]);
        if (cancelled) return;
        setAgents(a);
        setNamespaces(ns);
      } catch (e) {
        if (!cancelled) setError(e instanceof ApiError ? e.message : "Erreur de chargement.");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    const id = setInterval(load, 10_000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);

  const total = agents.length;
  const connected = agents.filter((a) => a.connectionState === "connected").length;
  const drift = agents.filter((a) => a.configSync === "drifted" || a.configSync === "failed").length;

  return (
    <div className="page">
      <div className="page-header">
        <h1>Vue d&apos;ensemble de la flotte</h1>
        <p>Tous les collecteurs OpenTelemetry connectés, groupés par namespace.</p>
      </div>

      {error && <div className="error-banner">{error}</div>}
      {loading && !error && <div className="empty-state">Chargement…</div>}

      {!loading && !error && (
        <>
          <div className="kpi-row">
            <Kpi label="Total agents" value={total} />
            <Kpi label="Connectés" value={connected} />
            <Kpi label="Namespaces" value={namespaces.length} />
            <Kpi label="Dérive de config" value={drift} />
          </div>

          <div className="page-header" style={{ marginTop: 22 }}>
            <p style={{ textTransform: "uppercase", fontSize: 11, letterSpacing: "0.04em" }}>
              Fleets par namespace
            </p>
          </div>
          {namespaces.length === 0 ? (
            <div className="empty-state">Aucun agent connecté pour le moment.</div>
          ) : (
            <div className="fleet-grid">
              {namespaces.map((ns) => (
                <div key={ns.namespace} className="panel fleet-card" onClick={() => onOpenNamespace(ns.namespace)}>
                  <div className="ns-name mono">{ns.namespace}</div>
                  <div className="total">{ns.total}</div>
                  <div style={{ color: "var(--text-secondary)", fontSize: 11 }}>agents</div>
                  <div className="stacked-bar">
                    {ns.connected > 0 && <div className="connected" style={{ flex: ns.connected }} />}
                    {ns.stale > 0 && <div className="stale" style={{ flex: ns.stale }} />}
                    {ns.disconnected > 0 && <div className="disconnected" style={{ flex: ns.disconnected }} />}
                  </div>
                  <div style={{ fontSize: 11, color: "var(--text-secondary)" }}>
                    {ns.connected} connecté(s) · {ns.configSynced}/{ns.total} synchronisées
                  </div>
                </div>
              ))}
            </div>
          )}

          <div className="page-header" style={{ marginTop: 22 }}>
            <p style={{ textTransform: "uppercase", fontSize: 11, letterSpacing: "0.04em" }}>
              Agents récemment vus
            </p>
          </div>
          <div className="panel">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Service</th>
                  <th>Namespace</th>
                  <th>Statut</th>
                  <th>Vu</th>
                </tr>
              </thead>
              <tbody>
                {agents.slice(0, 8).map((a) => (
                  <tr key={a.instanceUid} onClick={() => onOpenAgent(a.instanceUid)}>
                    <td>{a.serviceName || a.podName || a.instanceUid.slice(0, 8)}</td>
                    <td className="mono">{a.namespace || "—"}</td>
                    <td>
                      <span className={`badge ${a.connectionState === "connected" ? "success" : a.connectionState === "stale" ? "warning" : "danger"}`}>
                        {a.connectionState}
                      </span>
                    </td>
                    <td className="mono">{a.lastSeen ? new Date(a.lastSeen).toLocaleTimeString() : "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            {agents.length === 0 && <div className="empty-state">Aucun agent.</div>}
          </div>
        </>
      )}
    </div>
  );
}

function Kpi({ label, value }: { label: string; value: number }) {
  return (
    <div className="panel kpi-card">
      <div className="label">{label}</div>
      <div className="value mono">{value}</div>
    </div>
  );
}
