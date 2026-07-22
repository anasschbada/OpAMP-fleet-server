import { useEffect, useState } from "react";
import { getAgent, getAgentMetrics, listConfigPushes, pushConfig } from "../api/endpoints";
import type { AgentDetail as AgentDetailType, ConfigPush, MetricsSnapshot } from "../types";
import { ApiError } from "../api/client";
import { bytes, configSyncBadgeClass, configSyncLabel, connectionBadgeClass, connectionLabel, percent, relativeTime, uptime } from "../format";
import { Sparkline } from "./Sparkline";
import { diffLines } from "../diff";

type Tab = "metrics" | "config" | "history";
type PushState = "idle" | "confirm" | "pushing" | "success";

export function AgentDetail({ uid, onBack }: { uid: string; onBack: () => void }) {
  const [agent, setAgent] = useState<AgentDetailType | null>(null);
  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null);
  const [pushes, setPushes] = useState<ConfigPush[]>([]);
  const [tab, setTab] = useState<Tab>("metrics");
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [draftYaml, setDraftYaml] = useState("");
  const [pushState, setPushState] = useState<PushState>("idle");
  const [pushError, setPushError] = useState<string | null>(null);
  const [selectedPushId, setSelectedPushId] = useState<string | null>(null);

  async function reload() {
    try {
      const [a, m, p] = await Promise.all([getAgent(uid), getAgentMetrics(uid), listConfigPushes(uid)]);
      setAgent(a);
      setMetrics(m);
      setPushes(p);
      if (!selectedPushId && p.length > 0) setSelectedPushId(p[p.length - 1].id);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Erreur de chargement.");
    }
  }

  useEffect(() => {
    reload();
    const id = setInterval(reload, 5_000);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [uid]);

  if (error) return <div className="page"><div className="error-banner">{error}</div></div>;
  if (!agent) return <div className="page"><div className="empty-state">Chargement…</div></div>;

  function startEdit() {
    setDraftYaml(agent!.effectiveConfigYaml);
    setEditing(true);
    setPushState("idle");
  }

  function cancelEdit() {
    setEditing(false);
    setPushState("idle");
  }

  async function confirmPush() {
    setPushState("pushing");
    setPushError(null);
    try {
      await pushConfig(uid, draftYaml, "Modification manuelle depuis l'UI");
      setPushState("success");
      await reload();
      setTimeout(() => {
        setPushState("idle");
        setEditing(false);
      }, 2500);
    } catch (e) {
      setPushError(e instanceof ApiError ? e.message : "Échec de l'envoi.");
      setPushState("idle");
    }
  }

  return (
    <div className="page">
      <a onClick={onBack} style={{ cursor: "pointer", fontSize: 12.5, color: "var(--text-secondary)" }}>
        ← Retour
      </a>

      <div className="panel detail-header" style={{ marginTop: 10 }}>
        <div className="identity">
          <div className="icon-tile">
            <span className="status-dot" style={{ color: `var(--${connectionBadgeClass[agent.connectionState]})` }} />
          </div>
          <div>
            <div style={{ fontWeight: 600, fontSize: 16 }}>{agent.serviceName || agent.podName || agent.instanceUid}</div>
            <div className="mono" style={{ color: "var(--text-secondary)", fontSize: 12 }}>
              {agent.namespace || "—"} · {agent.podName || "—"} · v{agent.version || "?"}
            </div>
          </div>
        </div>
        <div style={{ display: "flex", gap: 24, textAlign: "right" }}>
          <Stat label="Statut" value={connectionLabel[agent.connectionState]} />
          <Stat label="Uptime" value={uptime(agent.startTime)} />
          <Stat label="Nœud" value={agent.nodeName || "—"} />
        </div>
      </div>

      <div className="tab-bar">
        <button className={tab === "metrics" ? "active" : ""} onClick={() => setTab("metrics")}>Métriques</button>
        <button className={tab === "config" ? "active" : ""} onClick={() => setTab("config")}>Configuration</button>
        <button className={tab === "history" ? "active" : ""} onClick={() => setTab("history")}>Historique</button>
      </div>

      {tab === "metrics" && (
        <div className="metrics-grid">
          <MetricCard label="CPU" unit="s/s" points={metrics?.cpuSecondsPerSec ?? []} formatter={(v) => v.toFixed(3)} />
          <MetricCard label="Mémoire" unit="" points={metrics?.memoryRssBytes ?? []} formatter={bytes} />
          <MetricCard label="Points de données reçus" unit="/s" points={metrics?.receivedPointsRate ?? []} formatter={(v) => v.toFixed(0)} />
          <MetricCard label="Taux de succès d'export" unit="" points={metrics?.exportSuccessRatio ?? []} formatter={percent} />
        </div>
      )}

      {tab === "config" && (
        <div className="panel" style={{ padding: 20 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
            <div>
              <div style={{ fontWeight: 600 }}>Configuration effective</div>
              <div style={{ fontSize: 12, color: "var(--text-secondary)" }}>
                <span className={`badge ${configSyncBadgeClass[agent.configSync]}`} style={{ marginRight: 8 }}>
                  {configSyncLabel[agent.configSync]}
                </span>
                {agent.pushedBy && <>poussée par {agent.pushedBy} {relativeTime(agent.lastPushedAt)}</>}
              </div>
            </div>
            <div style={{ display: "flex", gap: 8 }}>
              {!editing && (
                <button className="btn accent-soft" onClick={startEdit}>Modifier</button>
              )}
              {editing && pushState === "idle" && (
                <>
                  <button className="btn ghost" onClick={cancelEdit}>Annuler</button>
                  <button className="btn accent-solid" onClick={() => setPushState("confirm")}>Pousser la config</button>
                </>
              )}
            </div>
          </div>

          {pushError && <div className="error-banner">{pushError}</div>}

          {editing && pushState === "confirm" && (
            <div className="state-bar warning">
              <span>Confirmer l&apos;envoi de cette configuration à l&apos;agent ?</span>
              <div style={{ display: "flex", gap: 8 }}>
                <button className="btn ghost" onClick={() => setPushState("idle")}>Annuler</button>
                <button className="btn accent-solid" onClick={confirmPush}>Confirmer l&apos;envoi</button>
              </div>
            </div>
          )}
          {pushState === "pushing" && (
            <div className="state-bar accent">Envoi de la configuration via WebSocket OpAMP…</div>
          )}
          {pushState === "success" && (
            <div className="state-bar success">Configuration envoyée. En attente de confirmation par l&apos;agent.</div>
          )}

          <textarea
            className="yaml-editor mono"
            readOnly={!editing || pushState !== "idle"}
            value={editing ? draftYaml : agent.effectiveConfigYaml}
            onChange={(e) => setDraftYaml(e.target.value)}
            spellCheck={false}
          />
        </div>
      )}

      {tab === "history" && (
        <div className="history-layout">
          <div className="panel" style={{ padding: 14 }}>
            <div style={{ fontSize: 11, textTransform: "uppercase", color: "var(--text-tertiary)", marginBottom: 8 }}>
              Configurations poussées
            </div>
            <ul className="push-list">
              {pushes.map((p) => (
                <li key={p.id} className={p.id === selectedPushId ? "selected" : ""} onClick={() => setSelectedPushId(p.id)}>
                  <div className="mono" style={{ fontSize: 11.5 }}>{new Date(p.timestamp).toLocaleString()}</div>
                  <span className={`badge ${p.succeeded ? "success" : "warning"}`} style={{ marginTop: 4 }}>
                    {p.succeeded ? "Succès" : "En attente / Échec"}
                  </span>
                  <div style={{ fontSize: 11, color: "var(--text-secondary)" }}>{p.pushedBy}</div>
                </li>
              ))}
              {pushes.length === 0 && <div className="empty-state">Aucune configuration poussée pour le moment.</div>}
            </ul>
          </div>
          <div className="panel" style={{ padding: 14 }}>
            <HistoryDiff pushes={pushes} selectedId={selectedPushId} />
          </div>
        </div>
      )}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div style={{ fontSize: 10.5, color: "var(--text-tertiary)" }}>{label}</div>
      <div style={{ fontSize: 13 }}>{value}</div>
    </div>
  );
}

function MetricCard({
  label,
  unit,
  points,
  formatter,
}: {
  label: string;
  unit: string;
  points: { time: string; value: number }[];
  formatter: (v: number) => string;
}) {
  const last = points.length > 0 ? points[points.length - 1].value : 0;
  return (
    <div className="panel metric-card">
      <div style={{ fontSize: 12, color: "var(--text-secondary)" }}>{label}</div>
      <div className="metric-value mono">
        {formatter(last)}
        {unit && <span style={{ fontSize: 14, color: "var(--text-secondary)" }}> {unit}</span>}
      </div>
      <Sparkline points={points} />
      <div style={{ fontSize: 10.5, color: "var(--text-tertiary)", marginTop: 6 }}>30 derniers points relevés</div>
    </div>
  );
}

function HistoryDiff({ pushes, selectedId }: { pushes: ConfigPush[]; selectedId: string | null }) {
  const idx = pushes.findIndex((p) => p.id === selectedId);
  if (idx === -1) return <div className="empty-state">Sélectionnez une entrée.</div>;
  const current = pushes[idx];
  const previous = idx > 0 ? pushes[idx - 1] : null;
  const lines = diffLines(previous?.configYaml ?? "", current.configYaml);
  const added = lines.filter((l) => l.type === "add").length;
  const removed = lines.filter((l) => l.type === "del").length;

  return (
    <div>
      <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 10 }}>
        <div style={{ fontWeight: 600 }}>{previous ? "Diff vs version précédente" : "Version initiale"}</div>
        <div className="mono" style={{ fontSize: 12 }}>
          <span style={{ color: "var(--success)" }}>+{added}</span>{" "}
          <span style={{ color: "var(--danger)" }}>−{removed}</span>
        </div>
      </div>
      <div>
        {lines.map((l, i) => (
          <div key={i} className={`diff-line ${l.type === "add" ? "added" : l.type === "del" ? "removed" : ""}`}>
            {l.type === "add" ? "+ " : l.type === "del" ? "− " : "  "}
            {l.text}
          </div>
        ))}
      </div>
    </div>
  );
}
