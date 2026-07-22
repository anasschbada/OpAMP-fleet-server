import { useEffect, useMemo, useState } from "react";
import { getComponentCatalog, listAgents, pushConfig } from "../api/endpoints";
import type { Agent, Component, ComponentCatalog } from "../types";
import { ApiError } from "../api/client";
import { buildConfigYaml, emptySelection, GroupSelection, Signal, SIGNALS } from "../yamlBuilder";

type PushState = "idle" | "confirm" | "pushing" | "success";

const SIGNAL_LABELS: Record<Signal, string> = { logs: "Logs", metrics: "Metrics", traces: "Traces" };

export function ConfigBuilder() {
  const [catalog, setCatalog] = useState<ComponentCatalog | null>(null);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [namespace, setNamespace] = useState("");
  const [agentUid, setAgentUid] = useState("");
  const [error, setError] = useState<string | null>(null);

  const [signalsEnabled, setSignalsEnabled] = useState<Record<Signal, boolean>>({ logs: false, metrics: true, traces: false });
  const [selections, setSelections] = useState<Record<Signal, GroupSelection>>({
    logs: emptySelection(),
    metrics: emptySelection(),
    traces: emptySelection(),
  });

  const [pushState, setPushState] = useState<PushState>("idle");
  const [pushError, setPushError] = useState<string | null>(null);

  useEffect(() => {
    async function load() {
      try {
        const [c, a] = await Promise.all([getComponentCatalog(), listAgents()]);
        setCatalog(c);
        setAgents(a);
      } catch (e) {
        setError(e instanceof ApiError ? e.message : "Erreur de chargement.");
      }
    }
    load();
  }, []);

  const namespaces = useMemo(() => Array.from(new Set(agents.map((a) => a.namespace).filter(Boolean))).sort(), [agents]);
  const namespaceAgents = agents.filter((a) => !namespace || a.namespace === namespace);

  function toggleSignal(sig: Signal) {
    setSignalsEnabled((prev) => ({ ...prev, [sig]: !prev[sig] }));
  }

  function toggleComponent(sig: Signal, group: keyof GroupSelection, id: string) {
    setSelections((prev) => {
      const current = prev[sig][group];
      const next = current.includes(id) ? current.filter((x) => x !== id) : [...current, id];
      return { ...prev, [sig]: { ...prev[sig], [group]: next } };
    });
  }

  const generated = catalog ? buildConfigYaml(catalog, signalsEnabled, selections) : "";
  const isValidConfig = catalog !== null && !generated.startsWith("#");

  async function confirmApply() {
    if (!agentUid) return;
    setPushState("pushing");
    setPushError(null);
    try {
      await pushConfig(agentUid, generated, "Généré depuis l'éditeur de pipeline");
      setPushState("success");
      setTimeout(() => setPushState("idle"), 2500);
    } catch (e) {
      setPushError(e instanceof ApiError ? e.message : "Échec de l'envoi.");
      setPushState("idle");
    }
  }

  if (error) return <div className="page"><div className="error-banner">{error}</div></div>;
  if (!catalog) return <div className="page"><div className="empty-state">Chargement…</div></div>;

  return (
    <div className="page">
      <div className="page-header">
        <h1>Éditeur de pipeline</h1>
        <p>Une rangée par signal · activez Logs, Metrics et/ou Traces indépendamment.</p>
      </div>

      <div className="filter-row">
        <select value={namespace} onChange={(e) => { setNamespace(e.target.value); setAgentUid(""); }}>
          <option value="">Tous les namespaces</option>
          {namespaces.map((ns) => <option key={ns} value={ns}>{ns}</option>)}
        </select>
        <select value={agentUid} onChange={(e) => setAgentUid(e.target.value)}>
          <option value="">Sélectionner un agent…</option>
          {namespaceAgents.map((a) => (
            <option key={a.instanceUid} value={a.instanceUid}>
              {a.serviceName || a.podName || a.instanceUid.slice(0, 8)} ({a.namespace})
            </option>
          ))}
        </select>
      </div>

      {SIGNALS.map((sig) => (
        <div key={sig} className="panel signal-card">
          <label className="signal-header">
            <input type="checkbox" checked={signalsEnabled[sig]} onChange={() => toggleSignal(sig)} />
            {SIGNAL_LABELS[sig]}
          </label>

          {signalsEnabled[sig] && (
            <div className="component-grid">
              <ComponentColumn title="Receivers" group="receivers" sig={sig} items={catalog.receivers} selections={selections} onToggle={toggleComponent} />
              <ComponentColumn title="Processors" group="processors" sig={sig} items={catalog.processors} selections={selections} onToggle={toggleComponent} />
              <ComponentColumn title="Connectors" group="connectors" sig={sig} items={catalog.connectors} selections={selections} onToggle={toggleComponent} />
              <ComponentColumn title="Exporters" group="exporters" sig={sig} items={catalog.exporters} selections={selections} onToggle={toggleComponent} />
              <ComponentColumn title="Extensions" group="extensions" sig={sig} items={catalog.extensions} selections={selections} onToggle={toggleComponent} />
            </div>
          )}
        </div>
      ))}

      <div className="panel" style={{ padding: 20 }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
          <div style={{ fontWeight: 600 }}>Configuration générée</div>
          <div style={{ display: "flex", gap: 8 }}>
            {pushState === "idle" && (
              <button className="btn accent-soft" disabled={!agentUid || !isValidConfig} onClick={() => setPushState("confirm")}>
                Appliquer au collecteur
              </button>
            )}
          </div>
        </div>

        {!agentUid && <div style={{ fontSize: 12, color: "var(--text-secondary)", marginBottom: 10 }}>Sélectionnez un agent ci-dessus pour pouvoir appliquer cette configuration.</div>}
        {pushError && <div className="error-banner">{pushError}</div>}
        {pushState === "confirm" && (
          <div className="state-bar warning">
            <span>Appliquer cette configuration à l&apos;agent sélectionné ?</span>
            <div style={{ display: "flex", gap: 8 }}>
              <button className="btn ghost" onClick={() => setPushState("idle")}>Annuler</button>
              <button className="btn accent-solid" onClick={confirmApply}>Confirmer</button>
            </div>
          </div>
        )}
        {pushState === "pushing" && <div className="state-bar accent">Envoi…</div>}
        {pushState === "success" && <div className="state-bar success">✓ Appliquée avec succès.</div>}

        <textarea className="yaml-editor mono" readOnly value={generated} spellCheck={false} />
      </div>
    </div>
  );
}

function ComponentColumn({
  title,
  group,
  sig,
  items,
  selections,
  onToggle,
}: {
  title: string;
  group: keyof GroupSelection;
  sig: Signal;
  items: Component[];
  selections: Record<Signal, GroupSelection>;
  onToggle: (sig: Signal, group: keyof GroupSelection, id: string) => void;
}) {
  const applicable = items.filter((c) => c.signals.includes(sig));
  const selected = selections[sig][group];
  return (
    <div className="component-column">
      <h4>{title}</h4>
      {applicable.map((c) => (
        <label key={c.id} className="checkbox-row">
          <input type="checkbox" checked={selected.includes(c.id)} onChange={() => onToggle(sig, group, c.id)} />
          {c.label}
        </label>
      ))}
      {applicable.length === 0 && <div style={{ fontSize: 11, color: "var(--text-tertiary)" }}>—</div>}
    </div>
  );
}
