import { useEffect, useState } from "react";
import { TopBar, View } from "./components/TopBar";
import { Overview } from "./components/Overview";
import { AgentsList } from "./components/AgentsList";
import { AgentDetail } from "./components/AgentDetail";
import { ConfigBuilder } from "./components/ConfigBuilder";
import { Login } from "./components/Login";
import { clearToken, getToken } from "./api/client";

type Theme = "dark" | "light";

export function App() {
  const [authed, setAuthed] = useState(() => getToken() !== null);
  const [theme, setTheme] = useState<Theme>(() => (localStorage.getItem("opamp-fleet-ui:theme") as Theme) ?? "dark");
  const [view, setView] = useState<View>("fleets");
  const [namespaceFilter, setNamespaceFilter] = useState<string | null>(null);
  const [selectedAgentUid, setSelectedAgentUid] = useState<string | null>(null);
  const [previousView, setPreviousView] = useState<View>("fleets");

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
    localStorage.setItem("opamp-fleet-ui:theme", theme);
  }, [theme]);

  if (!authed) {
    return <Login onSubmit={() => setAuthed(true)} />;
  }

  function openAgent(uid: string, fromView: View) {
    setSelectedAgentUid(uid);
    setPreviousView(fromView);
  }

  function logout() {
    clearToken();
    setAuthed(false);
  }

  return (
    <div className="app">
      <TopBar
        view={view}
        onNavigate={(v) => {
          setSelectedAgentUid(null);
          setNamespaceFilter(null);
          setView(v);
        }}
        theme={theme}
        onToggleTheme={() => setTheme((t) => (t === "dark" ? "light" : "dark"))}
        onLogout={logout}
      />
      <main style={{ flex: 1, overflow: "auto" }}>
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
            onOpenNamespace={(ns) => {
              setNamespaceFilter(ns);
              setView("list");
            }}
            onOpenAgent={(uid) => openAgent(uid, "fleets")}
          />
        ) : view === "list" ? (
          <AgentsList initialNamespace={namespaceFilter} onOpenAgent={(uid) => openAgent(uid, "list")} />
        ) : (
          <ConfigBuilder />
        )}
      </main>
    </div>
  );
}
