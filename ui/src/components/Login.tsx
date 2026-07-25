import { useState } from "react";
import { setBasicAuth, setBearerToken } from "../api/client";

type Mode = "token" | "basic";

// There is no user database on the server (see docs/RBAC.md): everyone who
// operates this UI shares one of a small set of API bearer tokens, OR (if
// the server has HTTP Basic Auth configured -- see internal/config.Config's
// BasicAuthUsernameFile/BasicAuthPasswordFile) one shared username/password
// pair. Both are additive login methods against the same REST API, never a
// per-user session; this screen just captures whichever one the operator
// has, once per browser tab (sessionStorage, see api/client.ts). It makes
// no claim of per-user identity beyond whatever X-Forwarded-User header a
// reverse proxy in front of this UI may add (see resolvePushedBy in
// internal/api/agents.go).
export function Login({ onSubmit }: { onSubmit: () => void }) {
  const [mode, setMode] = useState<Mode>("token");
  const [token, setToken] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (mode === "token") {
      if (!token.trim()) return;
      setBearerToken(token.trim());
    } else {
      if (!username.trim() || !password) return;
      setBasicAuth(username.trim(), password);
    }
    onSubmit();
  }

  return (
    <div className="login-screen">
      <form className="login-card panel" onSubmit={submit}>
        <div className="wordmark">OpAMP Fleet</div>

        <div className="view-toggle" style={{ width: "100%", marginTop: 14 }}>
          <button type="button" className={mode === "token" ? "active" : ""} onClick={() => setMode("token")}>
            Jeton d&apos;accès
          </button>
          <button type="button" className={mode === "basic" ? "active" : ""} onClick={() => setMode("basic")}>
            Identifiants
          </button>
        </div>

        {mode === "token" ? (
          <>
            <p style={{ color: "var(--text-secondary)", fontSize: 12.5 }}>
              Entrez le jeton d&apos;accès fourni par l&apos;administrateur de la flotte.
            </p>
            <input
              type="password"
              autoFocus
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder="Jeton d'accès"
            />
          </>
        ) : (
          <>
            <p style={{ color: "var(--text-secondary)", fontSize: 12.5 }}>
              Entrez le nom d&apos;utilisateur et le mot de passe fournis par
              l&apos;administrateur de la flotte.
            </p>
            <input
              type="text"
              autoFocus
              autoComplete="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="Nom d'utilisateur"
            />
            <input
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Mot de passe"
            />
          </>
        )}

        <button type="submit" className="btn accent-solid" style={{ width: "100%" }}>
          Continuer
        </button>
      </form>
    </div>
  );
}
