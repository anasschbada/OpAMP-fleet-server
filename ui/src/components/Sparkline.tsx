import type { MetricPoint } from "../types";

// Minimal inline SVG sparkline -- no charting library dependency for what
// is fundamentally one polyline.
export function Sparkline({ points, width = 240, height = 64 }: { points: MetricPoint[]; width?: number; height?: number }) {
  if (points.length < 2) {
    return <div style={{ height, color: "var(--text-tertiary)", fontSize: 11 }}>Pas encore assez de données.</div>;
  }
  const values = points.map((p) => p.value);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;
  const step = width / (points.length - 1);
  const coords = values.map((v, i) => `${(i * step).toFixed(1)},${(height - ((v - min) / range) * height).toFixed(1)}`);
  const line = `M${coords.join(" L")}`;
  const area = `${line} L${width},${height} L0,${height} Z`;

  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`}>
      <path d={area} fill="var(--accent-soft)" stroke="none" />
      <path d={line} fill="none" stroke="var(--accent)" strokeWidth={1.5} />
    </svg>
  );
}
