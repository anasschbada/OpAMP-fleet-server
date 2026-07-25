import { connectionBadgeClass, connectionLabel, relativeTime } from "../../format";
import type { Agent } from "../../types";

const VISIBLE_CAP = 8;

export function RecentActivity({ agents, onOpenAgent }: { agents: Agent[]; onOpenAgent: (uid: string) => void }) {
  const recent = [...agents]
    .sort((a, b) => (b.lastSeen ?? "").localeCompare(a.lastSeen ?? ""))
    .slice(0, VISIBLE_CAP);

  return (
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
          {recent.map((a) => (
            <tr key={a.instanceUid} onClick={() => onOpenAgent(a.instanceUid)}>
              <td>{a.serviceName || a.podName || a.instanceUid.slice(0, 8)}</td>
              <td className="mono">{a.namespace || "—"}</td>
              <td>
                <span className={`badge ${connectionBadgeClass[a.connectionState]}`}>{connectionLabel[a.connectionState]}</span>
              </td>
              <td className="mono">{relativeTime(a.lastSeen)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {recent.length === 0 && <div className="empty-state">Aucun agent.</div>}
    </div>
  );
}
