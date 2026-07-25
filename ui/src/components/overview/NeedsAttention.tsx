import { useState } from "react";
import { deriveProblems } from "../../problems";
import { ProblemList } from "../ProblemList";
import type { Agent } from "../../types";

const VISIBLE_CAP = 3;

// Acknowledging a problem only hides it from this session's view -- there
// is no server-side "acknowledged" concept to persist it against, so it
// resets on reload. Beyond VISIBLE_CAP entries, the rest are one click away
// on the full Alerts page rather than pushed here.
export function NeedsAttention({
  agents,
  onOpenNamespace,
  onOpenAlerts,
}: {
  agents: Agent[];
  onOpenNamespace: (ns: string) => void;
  onOpenAlerts: () => void;
}) {
  const [acked, setAcked] = useState<Set<string>>(new Set());
  const [showAcked, setShowAcked] = useState(false);

  const problems = deriveProblems(agents);
  const unhidden = showAcked ? problems : problems.filter((p) => !acked.has(p.id));
  const shown = unhidden.slice(0, VISIBLE_CAP);
  const hiddenCount = unhidden.length - shown.length;

  function toggleAck(id: string) {
    setAcked((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  return (
    <div className="panel">
      <ProblemList
        visible={shown}
        acked={acked}
        showAcked={showAcked}
        onToggleShowAcked={setShowAcked}
        onToggleAck={toggleAck}
        onOpenNamespace={onOpenNamespace}
      />
      {hiddenCount > 0 && (
        <a className="link-more" onClick={onOpenAlerts}>
          Voir {hiddenCount} de plus →
        </a>
      )}
    </div>
  );
}
