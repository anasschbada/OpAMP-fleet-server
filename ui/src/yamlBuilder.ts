import type { Component, ComponentCatalog } from "./types";

export type Signal = "logs" | "metrics" | "traces";
export const SIGNALS: Signal[] = ["logs", "metrics", "traces"];

export interface GroupSelection {
  receivers: string[];
  processors: string[];
  connectors: string[];
  exporters: string[];
  extensions: string[];
}

export function emptySelection(): GroupSelection {
  return { receivers: [], processors: [], connectors: [], exporters: [], extensions: [] };
}

function indent(yaml: string): string {
  return yaml
    .split("\n")
    .filter((l) => l.length > 0)
    .map((l) => "  " + l)
    .join("\n");
}

function renderBlock(catalog: Component[], ids: Set<string>): string {
  let out = "";
  for (const id of ids) {
    const comp = catalog.find((c) => c.id === id);
    out += comp ? indent(comp.yaml) + "\n" : `  ${id}:\n`;
  }
  return out;
}

// buildConfigYaml mirrors the design prototype's buildFleetConfigYaml
// (mockData.js): union every enabled signal's selected components into
// single receivers:/processors:/connectors:/exporters:/extensions: blocks
// (the OTel Collector config format requires exactly one instance of each
// component, referenced per-pipeline), then emit one
// service.pipelines.<signal> block per enabled signal referencing only
// that signal's own selected ids. Connectors act as additional exporters
// on the emitting side of a pipeline, matching how OTel connectors work.
export function buildConfigYaml(
  catalog: ComponentCatalog,
  signalsEnabled: Record<Signal, boolean>,
  selections: Record<Signal, GroupSelection>,
): string {
  const enabled = SIGNALS.filter((s) => signalsEnabled[s]);
  if (enabled.length === 0) {
    return "# Activez au moins un signal (Logs, Metrics, Traces) pour générer une configuration.";
  }
  for (const sig of enabled) {
    const sel = selections[sig];
    if (sel.receivers.length === 0 || sel.exporters.length === 0) {
      return `# Sélectionnez au moins un receiver et un exporter pour le signal "${sig}".`;
    }
  }

  const allReceivers = new Set<string>();
  const allProcessors = new Set<string>();
  const allConnectors = new Set<string>();
  const allExporters = new Set<string>();
  const allExtensions = new Set<string>();
  for (const sig of enabled) {
    const sel = selections[sig];
    sel.receivers.forEach((id) => allReceivers.add(id));
    sel.processors.forEach((id) => allProcessors.add(id));
    sel.connectors.forEach((id) => allConnectors.add(id));
    sel.exporters.forEach((id) => allExporters.add(id));
    sel.extensions.forEach((id) => allExtensions.add(id));
  }

  let out = "";
  if (allExtensions.size > 0) {
    out += "extensions:\n" + renderBlock(catalog.extensions, allExtensions) + "\n";
  }
  out += "receivers:\n" + renderBlock(catalog.receivers, allReceivers) + "\n";
  if (allProcessors.size > 0) {
    out += "processors:\n" + renderBlock(catalog.processors, allProcessors) + "\n";
  }
  if (allConnectors.size > 0) {
    out += "connectors:\n" + renderBlock(catalog.connectors, allConnectors) + "\n";
  }
  out += "exporters:\n" + renderBlock(catalog.exporters, allExporters) + "\n";

  out += "service:\n";
  if (allExtensions.size > 0) {
    out += `  extensions: [${[...allExtensions].join(", ")}]\n`;
  }
  out += "  pipelines:\n";
  for (const sig of enabled) {
    const sel = selections[sig];
    const exporters = [...sel.exporters, ...sel.connectors];
    out += `    ${sig}:\n`;
    out += `      receivers: [${sel.receivers.join(", ")}]\n`;
    if (sel.processors.length > 0) {
      out += `      processors: [${sel.processors.join(", ")}]\n`;
    }
    out += `      exporters: [${exporters.join(", ")}]\n`;
  }
  return out;
}
