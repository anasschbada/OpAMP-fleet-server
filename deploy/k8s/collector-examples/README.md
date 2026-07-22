# Example OpenTelemetry Collector manifests (any distribution)

These are **reference examples**, not a component this project deploys for
you: your collectors likely already exist, in namespaces this project has
no access to and makes no assumptions about. Adapt them to whatever
namespace/config your team owns.

They apply to **any OpenTelemetry Collector build that includes the
`opamp` extension** (upstream `opentelemetry-collector-contrib`, a vendor
distribution, or a custom build) -- nothing here is specific to one
vendor. The exact YAML keys under the `opamp` extension block can differ
slightly between collector-contrib versions; check
`otelcol-contrib components` / your distribution's docs for the version
you actually run before copying this verbatim.

## Files

- `configmap-collector-config.yaml` -- a starter collector config: OTLP
  receiver, batch/memory_limiter processors, the `opamp` extension pointed
  at the fleet server, and OTLP export to your existing observability
  backend (edit the exporter endpoint).
- `daemonset-agent.yaml` -- one collector per node (the "agent" role): adds
  `hostmetrics`, needs read-only host mounts for `/proc` and `/sys`.
- `deployment-gateway.yaml` -- a cluster-level aggregator (the "gateway"
  role): receives from the per-node agents over OTLP, no host access
  needed.
- `secret-opamp-auth.example.yaml` -- example Secret carrying the bearer
  token collectors present to the fleet server (see
  `deploy/k8s/platform/04-secret-auth-tokens.example.yaml` -- this must be
  one of the same tokens the server accepts).

## Why there's no Kubernetes RBAC here at all

These manifests deliberately identify the collector to the fleet server
using **Kubernetes Downward API environment variables** (`metadata.namespace`,
`metadata.name`, `spec.nodeName` injected as env vars, then folded into
`OTEL_RESOURCE_ATTRIBUTES` and picked up by the `resourcedetection`
processor's `env` detector) -- not the `k8sattributes` processor's
pod-enrichment feature.

That distinction matters: `k8sattributes`, when used to *enrich* telemetry
by watching the whole cluster's pods, needs `list`/`watch` on `pods` across
every namespace, which is a cluster-scoped access pattern only a
`ClusterRole` can grant. Downward API env vars need **no Kubernetes API
call at all** -- the kubelet injects them at pod start -- so a collector
only identifying *itself* (its own namespace/pod/node) never needs any
RBAC, matching the same "no ClusterRole, ideally no Role either" constraint
the server itself follows (see `../../docs/RBAC.md`).

If your team's telemetry pipeline separately needs `k8sattributes` for
enriching traces/logs/metrics coming from *other* pods, that's a
platform-level RBAC decision your team makes independently of this
project, and is out of scope here.

## `hostmetrics` and Pod Security Admission

The `daemonset-agent.yaml` example mounts the host's `/proc` and `/sys`
read-only so the `hostmetrics` receiver can scrape CPU/memory/disk. If your
namespace enforces the `restricted` Pod Security Standard, this will be
rejected -- that's a Pod Security Admission label on the namespace, not an
RBAC/API-server permission, and setting it requires whoever administers
that label (ask your platform team to label the namespace `baseline`, or
drop `hostmetrics` and the host mounts from the example if you can't get
that exception).
