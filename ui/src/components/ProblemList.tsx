import { severityBadgeClass, severityLabel } from "../problems";
import type { Problem } from "../problems";

// Shared rendering for a list of derived problems -- used by both
// NeedsAttention (Overview's capped feed) and Alerts (the full page), so
// the acknowledge control and empty state only exist in one place.
export function ProblemList({
  visible,
  acked,
  showAcked,
  onToggleShowAcked,
  onToggleAck,
  onOpenNamespace,
}: {
  visible: Problem[];
  acked: Set<string>;
  showAcked: boolean;
  onToggleShowAcked: (value: boolean) => void;
  onToggleAck: (id: string) => void;
  onOpenNamespace: (ns: string) => void;
}) {
  return (
    <>
      <div className="problem-panel-head">
        <label className="checkbox-row">
          <input type="checkbox" checked={showAcked} onChange={(e) => onToggleShowAcked(e.target.checked)} />
          Afficher les éléments acquittés
        </label>
      </div>

      {visible.length === 0 ? (
        <div className="empty-state">Tout semble normal — aucun problème détecté.</div>
      ) : (
        <ul className="problem-list">
          {visible.map((p) => (
            <li key={p.id} className="problem-row">
              <span className={`badge ${severityBadgeClass[p.severity]}`}>{severityLabel[p.severity]}</span>
              <div className="problem-body">
                <div className="problem-title" onClick={() => onOpenNamespace(p.namespace)}>
                  {p.title}
                </div>
                <div className="problem-detail">{p.detail}</div>
              </div>
              <button className="ack-btn" onClick={() => onToggleAck(p.id)}>
                {acked.has(p.id) ? "Rétablir" : "Acquitter"}
              </button>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}
