import { useEffect, useMemo, useState } from "react";
import { Sidebar, View } from "./components/Sidebar";
import { Overview } from "./components/Overview";
import { Alerts } from "./components/Alerts";
import { AgentsList } from "./components/AgentsList";
import { AgentDetail } from "./components/AgentDetail";
import { ConfigBuilder } from "./components/ConfigBuilder";
import { Login } from "./components/Login";
import { clearToken, getToken } from "./api/client";
import { useFleetData } from "./hooks/useFleetData";
import { deriveProblems } from "./problems";

type Theme = "dark" | "light";
const THEME_STORAGE_KEY = "opamp-fleet-ui:theme";

function initialTheme(): Theme {
  const stored = localStorage.getItem(THEME_STORAGE_KEY);
  if (stored === "dark" || stored === "light") return stored;
  // No explicit choice saved yet: default to whatever the browser/OS
  // prefers instead of hardcoding dark, so first paint already matches.
  return window.matchMedia?.("(prefers-color-scheme: light)").matches ? "light" : "dark";
}

export function App() {
  const [authed, setAuthed] = useState(() => getToken() !== null);
  const [theme, setTheme] = useState<Theme>(initialTheme);
  const [view, setView] = useState<View>("fleets");
  const [namespaceFilter, setNamespaceFilter] = useState<string | null>(null);
  const [selectedAgentUid, setSelectedAgentUid] = useState<string | null>(null);
  const [previousView, setPreviousView] = useState<View>("fleets");

  // Fetched once here and shared by every fleet-wide view (Overview, Alerts,
  // AgentsList, and the sidebar's alert badge) instead of each polling
  // independently -- see hooks/useFleetData.ts.
  const { agents, namespaces, loading, error } = useFleetData();
  const problems = useMemo(() => deriveProblems(agents), [agents]);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
    localStorage.setItem(THEME_STORAGE_KEY, theme);
  }, [theme]);

  if (!authed) {
    return <Login onSubmit={() => setAuthed(true)} />;
  }

  function openAgent(uid: string, fromView: View) {
    setSelectedAgentUid(uid);
    setPreviousView(fromView);
  }

  function openNamespace(ns: string) {
    setNamespaceFilter(ns);
    setView("list");
  }

  function logout() {
    clearToken();
    setAuthed(false);
  }

  return (
    <div className="app">
      <Sidebar
        view={view}
        onNavigate={(v) => {
          setSelectedAgentUid(null);
          setNamespaceFilter(null);
          setView(v);
        }}
        theme={theme}
        onToggleTheme={() => setTheme((t) => (t === "dark" ? "light" : "dark"))}
        onLogout={logout}
        alertCount={problems.length}
      />
      <main className="main-content">
        {selectedAgentUid ? (
          <AgentDetail
            uid={selectedAgentUid}
            onBack={() => {
              setSelectedAgentUid(null);
              setView(previousView);
            }}
          />
        ) : view === "fleets" ? (
          <Overview
            agents={agents}
            namespaces={namespaces}
            loading={loading}
            error={error}
            onOpenNamespace={openNamespace}
            onOpenAgent={(uid) => openAgent(uid, "fleets")}
            onOpenAlerts={() => setView("alerts")}
          />
        ) : view === "alerts" ? (
          <Alerts agents={agents} loading={loading} error={error} onOpenNamespace={openNamespace} />
        ) : view === "list" ? (
          <AgentsList agents={agents} initialNamespace={namespaceFilter} onOpenAgent={(uid) => openAgent(uid, "list")} />
        ) : (
          <ConfigBuilder />
        )}
      </main>
    </div>
  );
}
