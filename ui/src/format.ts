export function relativeTime(iso?: string): string {
  if (!iso) return "—";
  const diffMs = Date.now() - new Date(iso).getTime();
  const s = Math.max(0, Math.floor(diffMs / 1000));
  if (s < 5) return "à l'instant";
  if (s < 60) return `il y a ${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `il y a ${m}min`;
  const h = Math.floor(m / 60);
  if (h < 24) return `il y a ${h}h`;
  const d = Math.floor(h / 24);
  return `il y a ${d}j`;
}

export function uptime(iso?: string): string {
  if (!iso) return "—";
  const s = Math.max(0, Math.floor((Date.now() - new Date(iso).getTime()) / 1000));
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (d > 0) return `${d}j ${h}h`;
  if (h > 0) return `${h}h ${m}min`;
  return `${m}min`;
}

export function bytes(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return "0 Mo";
  const mb = n / (1024 * 1024);
  if (mb < 1024) return `${mb.toFixed(0)} Mo`;
  return `${(mb / 1024).toFixed(1)} Go`;
}

export function percent(ratio: number): string {
  return `${(ratio * 100).toFixed(1)}%`;
}

export const connectionLabel: Record<string, string> = {
  connected: "Connecté",
  stale: "Stale",
  disconnected: "Déconnecté",
};

export const connectionBadgeClass: Record<string, string> = {
  connected: "success",
  stale: "warning",
  disconnected: "danger",
};

export const configSyncLabel: Record<string, string> = {
  synced: "Synchronisée",
  pending: "En attente",
  drifted: "Dérive détectée",
  failed: "Échec",
};

export const configSyncBadgeClass: Record<string, string> = {
  synced: "success",
  pending: "warning",
  drifted: "danger",
  failed: "danger",
};
