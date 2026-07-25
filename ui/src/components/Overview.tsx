import { CommandPalette } from "./CommandPalette";
import { KpiStrip } from "./overview/KpiStrip";
import { NeedsAttention } from "./overview/NeedsAttention";
import { FleetComposition } from "./overview/FleetComposition";
import { NamespaceStatus } from "./overview/NamespaceStatus";
import { RecentActivity } from "./overview/RecentActivity";
import type { Agent, NamespaceSummary } from "../types";

// Thin orchestrator: fetches nothing itself (agents/namespaces come from
// App.tsx's shared poll, see hooks/useFleetData.ts) and composes a set of
// independent, self-contained sections that each only need the slice of
// data their props say they need.
export function Overview({
  agents,
  namespaces,
  loading,
  error,
  onOpenNamespace,
  onOpenAgent,
  onOpenAlerts,
}: {
  agents: Agent[];
  namespaces: NamespaceSummary[];
  loading: boolean;
  error: string | null;
  onOpenNamespace: (ns: string) => void;
  onOpenAgent: (uid: string) => void;
  onOpenAlerts: () => void;
}) {
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
          <CommandPalette agents={agents} namespaces={namespaces} onOpenAgent={onOpenAgent} onOpenNamespace={onOpenNamespace} />

          <KpiStrip agents={agents} />

          <SectionHead title="À surveiller" />
          <NeedsAttention agents={agents} onOpenNamespace={onOpenNamespace} onOpenAlerts={onOpenAlerts} />

          <SectionHead title="Composition de la flotte" />
          <FleetComposition agents={agents} />

          <NamespaceStatus namespaces={namespaces} agents={agents} onOpenNamespace={onOpenNamespace} />

          <SectionHead title="Activité récente" />
          <RecentActivity agents={agents} onOpenAgent={onOpenAgent} />
        </>
      )}
    </div>
  );
}

// NamespaceStatus renders its own section-head (it needs the view toggle
// next to the title), so it doesn't use this helper.
function SectionHead({ title }: { title: string }) {
  return (
    <div className="section-head">
      <div className="section-label">{title}</div>
    </div>
  );
}
