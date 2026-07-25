import { useEffect, useRef, useState } from "react";
import { apiBaseUrl } from "../api/client";

export type View = "fleets" | "alerts" | "list" | "config-builder";

const NAV_ITEMS: { view: View; label: string; icon: JSX.Element }[] = [
  {
    view: "fleets",
    label: "Vue d'ensemble",
    icon: (
      <svg width="18" height="18" viewBox="0 0 20 20" fill="none" aria-hidden="true">
        <circle cx="6" cy="10" r="3.4" stroke="currentColor" strokeWidth="1.5" />
        <circle cx="14" cy="10" r="3.4" stroke="currentColor" strokeWidth="1.5" />
        <path d="M9.4 8.6h1.2M4.5 7.2L3 6M15.5 7.2L17 6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      </svg>
    ),
  },
  {
    view: "alerts",
    label: "Alertes",
    icon: (
      <svg width="18" height="18" viewBox="0 0 20 20" fill="none" aria-hidden="true">
        <path
          d="M10 3.3c-2.1 0-3.7 1.7-3.7 3.8v2.2c0 .6-.2 1.1-.6 1.5l-.7.8c-.4.4-.1 1.1.5 1.1h9c.6 0 .9-.7.5-1.1l-.7-.8c-.4-.4-.6-.9-.6-1.5V7.1c0-2.1-1.6-3.8-3.7-3.8z"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinejoin="round"
        />
        <path d="M8.5 15.2c.25.65.9 1.1 1.5 1.1s1.25-.45 1.5-1.1" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      </svg>
    ),
  },
  {
    view: "list",
    label: "Tous les agents",
    icon: (
      <svg width="18" height="18" viewBox="0 0 20 20" fill="none" aria-hidden="true">
        <rect x="3" y="4.5" width="14" height="3" rx="1" stroke="currentColor" strokeWidth="1.5" />
        <rect x="3" y="8.5" width="14" height="3" rx="1" stroke="currentColor" strokeWidth="1.5" />
        <rect x="3" y="12.5" width="14" height="3" rx="1" stroke="currentColor" strokeWidth="1.5" />
      </svg>
    ),
  },
  {
    view: "config-builder",
    label: "Configuration",
    icon: (
      <svg width="18" height="18" viewBox="0 0 20 20" fill="none" aria-hidden="true">
        <path d="M6 3v6M6 13v4M10 3v2M10 9v8M14 3v9M14 16v1" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        <circle cx="6" cy="10.5" r="1.6" fill="currentColor" />
        <circle cx="10" cy="7.5" r="1.6" fill="currentColor" />
        <circle cx="14" cy="13.5" r="1.6" fill="currentColor" />
      </svg>
    ),
  },
];

const MOON = (
  <path d="M15.5 12.3A6.5 6.5 0 018.2 4.6a6.5 6.5 0 107.3 7.7z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
);
const SUN = (
  <>
    <circle cx="10" cy="10" r="3.2" stroke="currentColor" strokeWidth="1.5" />
    <path
      d="M10 3v1.6M10 15.4V17M17 10h-1.6M4.6 10H3M15 5l-1.1 1.1M6.1 13.9L5 15M15 15l-1.1-1.1M6.1 6.1L5 5"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
    />
  </>
);

const SIDEBAR_COLLAPSED_KEY = "opamp-fleet-ui:sidebar-collapsed";

export function Sidebar({
  view,
  onNavigate,
  theme,
  onToggleTheme,
  onLogout,
  alertCount,
}: {
  view: View;
  onNavigate: (v: View) => void;
  theme: "dark" | "light";
  onToggleTheme: () => void;
  onLogout: () => void;
  alertCount: number;
}) {
  const [expanded, setExpanded] = useState(() => localStorage.getItem(SIDEBAR_COLLAPSED_KEY) !== "1");
  const [userMenuOpen, setUserMenuOpen] = useState(false);
  const userMenuRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    localStorage.setItem(SIDEBAR_COLLAPSED_KEY, expanded ? "0" : "1");
  }, [expanded]);

  useEffect(() => {
    function onDocClick(e: MouseEvent) {
      if (userMenuRef.current && !userMenuRef.current.contains(e.target as Node)) setUserMenuOpen(false);
    }
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") setUserMenuOpen(false);
    }
    document.addEventListener("click", onDocClick);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("click", onDocClick);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, []);

  return (
    <aside className={`sidebar${expanded ? " expanded" : ""}`}>
      <div className="sidebar-top">
        <div className="logo-mark" />
        <span className="sidebar-wordmark">OpAMP Fleet</span>
        <button
          className="sidebar-collapse-btn"
          onClick={() => setExpanded((v) => !v)}
          aria-label={expanded ? "Réduire le menu" : "Développer le menu"}
          aria-expanded={expanded}
        >
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" aria-hidden="true">
            <path d={expanded ? "M10 3L5 8l5 5" : "M6 3l5 5-5 5"} stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </button>
      </div>

      <nav className="nav-rail" aria-label="Navigation principale">
        {NAV_ITEMS.map((item) => (
          <button key={item.view} className={`nav-item${view === item.view ? " active" : ""}`} onClick={() => onNavigate(item.view)}>
            {item.icon}
            <span className="nav-label">{item.label}</span>
            {item.view === "alerts" && alertCount > 0 && (
              <span className="nav-badge">{alertCount > 99 ? "99+" : alertCount}</span>
            )}
            <span className="nav-tip" role="tooltip">
              {item.label}
            </span>
          </button>
        ))}
      </nav>

      <div className="sidebar-bottom">
        <button className="nav-item" onClick={onToggleTheme}>
          <svg width="18" height="18" viewBox="0 0 20 20" fill="none" aria-hidden="true">
            {theme === "dark" ? SUN : MOON}
          </svg>
          <span className="nav-label">{theme === "dark" ? "Thème clair" : "Thème sombre"}</span>
          <span className="nav-tip" role="tooltip">
            {theme === "dark" ? "Passer au thème clair" : "Passer au thème sombre"}
          </span>
        </button>

        <div className="user-popover-wrap" ref={userMenuRef}>
          <button
            className="nav-item"
            aria-haspopup="true"
            aria-expanded={userMenuOpen}
            onClick={(e) => {
              e.stopPropagation();
              setUserMenuOpen((v) => !v);
            }}
          >
            <span className="avatar-circle">U</span>
            <span className="nav-label">Compte</span>
            <span className="nav-tip" role="tooltip">
              Compte &amp; infos serveur
            </span>
          </button>
          {userMenuOpen && (
            <div className="panel user-popover">
              <div className="up-user">
                <div className="up-avatar-lg">U</div>
                <div>
                  <div className="up-name">Session en cours</div>
                  <div className="up-sub">Authentification via jeton porteur</div>
                </div>
              </div>
              <div className="up-divider" />
              <div className="up-row">
                <span>Point de terminaison API</span>
                <span className="mono">{apiBaseUrl()}</span>
              </div>
              <div className="up-divider" />
              <button className="btn ghost" style={{ width: "100%" }} onClick={onLogout}>
                Déconnexion
              </button>
            </div>
          )}
        </div>
      </div>
    </aside>
  );
}
