import { useState } from "react";
import { deriveProblems } from "../problems";
import { ProblemList } from "./ProblemList";
import type { Agent } from "../types";

// The uncapped counterpart to Overview's NeedsAttention panel -- same
// deriveProblems() call and the same ProblemList renderer, so the two can
// never show different numbers for the same fleet state. Receives agents
// as a prop rather than fetching its own copy: this and Overview share the
// one poll loop started in App.tsx.
export function Alerts({
  agents,
  loading,
  error,
  onOpenNamespace,
}: {
  agents: Agent[];
  loading: boolean;
  error: string | null;
  onOpenNamespace: (ns: string) => void;
}) {
  const [acked, setAcked] = useState<Set<string>>(new Set());
  const [showAcked, setShowAcked] = useState(false);

  const problems = deriveProblems(agents);
  const visible = showAcked ? problems : problems.filter((p) => !acked.has(p.id));

  function toggleAck(id: string) {
    setAcked((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1>Alertes</h1>
        <p>Tous les problèmes ouverts sur la flotte, les plus critiques en premier.</p>
      </div>

      {error && <div className="error-banner">{error}</div>}
      {loading && !error && <div className="empty-state">Chargement…</div>}

      {!loading && !error && (
        <div className="panel">
          <ProblemList
            visible={visible}
            acked={acked}
            showAcked={showAcked}
            onToggleShowAcked={setShowAcked}
            onToggleAck={toggleAck}
            onOpenNamespace={onOpenNamespace}
          />
        </div>
      )}
    </div>
  );
}
