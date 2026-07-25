import { useEffect, useState } from "react";
import { listAgents, listNamespaces } from "../api/endpoints";
import { ApiError } from "../api/client";
import type { Agent, NamespaceSummary } from "../types";

const POLL_INTERVAL_MS = 10_000;

export interface FleetData {
  agents: Agent[];
  namespaces: NamespaceSummary[];
  loading: boolean;
  error: string | null;
}

// Fleet-wide agents/namespaces, polled once and shared by every view that
// needs them (sidebar alert badge, overview, alerts page, agents list).
// Centralized here instead of each view fetching independently -- with a
// sidebar badge now reading the same data as the page content, duplicate
// pollers would drift out of sync with each other between ticks.
export function useFleetData(): FleetData {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [namespaces, setNamespaces] = useState<NamespaceSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const [a, ns] = await Promise.all([listAgents(), listNamespaces()]);
        if (cancelled) return;
        setAgents(a);
        setNamespaces(ns);
        setError(null);
      } catch (e) {
        if (!cancelled) setError(e instanceof ApiError ? e.message : "Erreur de chargement.");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    load();
    const id = setInterval(load, POLL_INTERVAL_MS);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);

  return { agents, namespaces, loading, error };
}
