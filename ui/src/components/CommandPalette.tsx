import { useEffect, useRef, useState } from "react";
import type { Agent, NamespaceSummary } from "../types";

const RESULT_CAP = 6;

type Item =
  | { kind: "namespace"; key: string; namespace: string; total: number }
  | { kind: "agent"; key: string; uid: string; label: string; namespace: string };

// Self-contained: renders its own trigger bar and, on demand, its own
// overlay. Search runs entirely client-side over the agents/namespaces
// this component is handed -- there is no search endpoint to call.
export function CommandPalette({
  agents,
  namespaces,
  onOpenAgent,
  onOpenNamespace,
}: {
  agents: Agent[];
  namespaces: NamespaceSummary[];
  onOpenAgent: (uid: string) => void;
  onOpenNamespace: (ns: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen(true);
      } else if (e.key === "Escape") {
        setOpen(false);
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  useEffect(() => {
    if (!open) return;
    setQuery("");
    setSelected(0);
    const raf = requestAnimationFrame(() => inputRef.current?.focus());
    return () => cancelAnimationFrame(raf);
  }, [open]);

  const items = buildItems(query, agents, namespaces);
  const nsItems = items.filter((i): i is Item & { kind: "namespace" } => i.kind === "namespace");
  const agentItems = items.filter((i): i is Item & { kind: "agent" } => i.kind === "agent");

  function activate(item: Item) {
    setOpen(false);
    if (item.kind === "namespace") onOpenNamespace(item.namespace);
    else onOpenAgent(item.uid);
  }

  return (
    <>
      <button className="palette-trigger" onClick={() => setOpen(true)}>
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <circle cx="7" cy="7" r="5.5" stroke="currentColor" strokeWidth="1.4" />
          <path d="M11 11L14.5 14.5" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
        </svg>
        <span>Rechercher un agent ou un namespace…</span>
        <span className="palette-kbd">⌘K</span>
      </button>

      {open && (
        <div className="palette-backdrop" onClick={() => setOpen(false)}>
          <div className="palette" onClick={(e) => e.stopPropagation()} role="dialog" aria-label="Recherche rapide">
            <input
              ref={inputRef}
              className="palette-input"
              placeholder="Rechercher un agent, un pod, un namespace…"
              value={query}
              onChange={(e) => {
                setQuery(e.target.value);
                setSelected(0);
              }}
              onKeyDown={(e) => {
                if (e.key === "ArrowDown") {
                  e.preventDefault();
                  setSelected((s) => Math.min(s + 1, items.length - 1));
                } else if (e.key === "ArrowUp") {
                  e.preventDefault();
                  setSelected((s) => Math.max(s - 1, 0));
                } else if (e.key === "Enter" && items[selected]) {
                  activate(items[selected]);
                }
              }}
            />
            <div className="palette-results" role="listbox">
              {items.length === 0 && <div className="palette-empty">Aucun résultat pour « {query} »</div>}

              {nsItems.length > 0 && (
                <div className="palette-group">
                  <div className="palette-group-label">Namespaces</div>
                  {nsItems.map((item) => (
                    <PaletteRow key={item.key} selected={items[selected]?.key === item.key} onClick={() => activate(item)}>
                      <span className="mono">{item.namespace}</span>
                      <span className="palette-meta">
                        {item.total} agent{item.total > 1 ? "s" : ""}
                      </span>
                    </PaletteRow>
                  ))}
                </div>
              )}

              {agentItems.length > 0 && (
                <div className="palette-group">
                  <div className="palette-group-label">Agents</div>
                  {agentItems.map((item) => (
                    <PaletteRow key={item.key} selected={items[selected]?.key === item.key} onClick={() => activate(item)}>
                      <span className="mono">{item.label}</span>
                      <span className="palette-meta">{item.namespace || "—"}</span>
                    </PaletteRow>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  );
}

function PaletteRow({ selected, onClick, children }: { selected: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <div className={`palette-item${selected ? " selected" : ""}`} role="option" aria-selected={selected} onClick={onClick}>
      {children}
    </div>
  );
}

function buildItems(query: string, agents: Agent[], namespaces: NamespaceSummary[]): Item[] {
  const q = query.trim().toLowerCase();

  const nsMatches = namespaces.filter((n) => !q || n.namespace.toLowerCase().includes(q)).slice(0, RESULT_CAP);

  const agentMatches = agents
    .filter((a) => {
      if (!q) return true;
      return (
        a.namespace.toLowerCase().includes(q) ||
        a.serviceName.toLowerCase().includes(q) ||
        a.podName.toLowerCase().includes(q)
      );
    })
    .slice(0, RESULT_CAP);

  return [
    ...nsMatches.map((n) => ({ kind: "namespace" as const, key: `ns:${n.namespace}`, namespace: n.namespace, total: n.total })),
    ...agentMatches.map((a) => ({
      kind: "agent" as const,
      key: `agent:${a.instanceUid}`,
      uid: a.instanceUid,
      label: a.serviceName || a.podName || a.instanceUid.slice(0, 8),
      namespace: a.namespace,
    })),
  ];
}
