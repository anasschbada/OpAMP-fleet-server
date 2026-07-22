import { useState } from "react";
import { setToken } from "../api/client";

// There is no user database on the server (see docs/RBAC.md): everyone who
// operates this UI shares one of a small set of bearer tokens issued out of
// band by whoever runs the fleet server. This screen just captures that
// token once per browser tab (sessionStorage, see api/client.ts) -- it is
// not a login system and makes no claim of per-user identity beyond
// whatever X-Forwarded-User header a reverse proxy in front of this UI may
// add (see resolvePushedBy in internal/api/agents.go).
export function Login({ onSubmit }: { onSubmit: () => void }) {
  const [value, setValue] = useState("");

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!value.trim()) return;
    setToken(value.trim());
    onSubmit();
  }

  return (
    <div className="login-screen">
      <form className="login-card panel" onSubmit={submit}>
        <div className="wordmark">OpAMP Fleet</div>
        <p style={{ color: "var(--text-secondary)", fontSize: 12.5 }}>
          Entrez le jeton d&apos;accès fourni par l&apos;administrateur de la flotte.
        </p>
        <input
          type="password"
          autoFocus
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder="Jeton d'accès"
        />
        <button type="submit" className="btn accent-solid" style={{ width: "100%" }}>
          Continuer
        </button>
      </form>
    </div>
  );
}
