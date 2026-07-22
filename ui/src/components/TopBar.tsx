export type View = "fleets" | "list" | "config-builder";

export function TopBar({
  view,
  onNavigate,
  theme,
  onToggleTheme,
  onLogout,
}: {
  view: View;
  onNavigate: (v: View) => void;
  theme: "dark" | "light";
  onToggleTheme: () => void;
  onLogout: () => void;
}) {
  return (
    <header className="topbar">
      <div className="topbar-left">
        <div className="logo-mark" />
        <span className="wordmark">OpAMP Fleet</span>
        <nav className="pill-group" style={{ marginLeft: 18 }}>
          <button className={view === "fleets" ? "active" : ""} onClick={() => onNavigate("fleets")}>
            Vue d&apos;ensemble
          </button>
          <button className={view === "list" ? "active" : ""} onClick={() => onNavigate("list")}>
            Tous les agents
          </button>
          <button
            className={view === "config-builder" ? "active" : ""}
            onClick={() => onNavigate("config-builder")}
          >
            Configuration
          </button>
        </nav>
      </div>
      <div className="topbar-right">
        <span className="status-chip">
          <span className="status-dot pulsing" />
          Serveur OpAMP en ligne
        </span>
        <span className="mono" style={{ color: "var(--text-tertiary)", fontSize: 11 }}>
          ws:4320 · rest:8080
        </span>
        <button className="btn ghost" onClick={onToggleTheme}>
          {theme === "dark" ? "Clair" : "Sombre"}
        </button>
        <button className="btn ghost" onClick={onLogout}>
          Déconnexion
        </button>
      </div>
    </header>
  );
}
