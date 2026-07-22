package api

// component describes one OpenTelemetry Collector pipeline component
// (receiver/processor/connector/exporter/extension) offered by the config
// builder UI. The catalog below only lists components that ship in the
// upstream opentelemetry-collector and opentelemetry-collector-contrib
// distributions -- nothing here is specific to any one vendor's build, so
// the same server/UI works whether a namespace runs plain contrib, a
// vendor distribution, or a custom collector build, as long as that build
// includes the listed component. Teams whose collector includes additional
// or different components use each column's "Autre…" custom-component input
// in the UI (see design_handoff's config builder) rather than needing a
// server-side catalog change for every possible build.
type component struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Signals []string `json:"signals"` // subset of "logs","metrics","traces" this component applies to
	YAML    string   `json:"yaml"`    // starter YAML block, editable per-pipeline in the UI
}

type componentCatalog struct {
	Receivers  []component `json:"receivers"`
	Processors []component `json:"processors"`
	Connectors []component `json:"connectors"`
	Exporters  []component `json:"exporters"`
	Extensions []component `json:"extensions"`
}

var defaultCatalog = componentCatalog{
	Receivers: []component{
		{ID: "otlp", Label: "OTLP", Signals: []string{"logs", "metrics", "traces"},
			YAML: "otlp:\n  protocols:\n    grpc:\n    http:\n"},
		{ID: "hostmetrics", Label: "Host Metrics", Signals: []string{"metrics"},
			YAML: "hostmetrics:\n  collection_interval: 30s\n  scrapers:\n    cpu:\n    memory:\n    disk:\n"},
		{ID: "kubeletstats", Label: "Kubelet Stats", Signals: []string{"metrics"},
			YAML: "kubeletstats:\n  collection_interval: 20s\n  auth_type: serviceAccount\n  endpoint: https://${env:K8S_NODE_NAME}:10250\n"},
		{ID: "k8s_cluster", Label: "Kubernetes Cluster", Signals: []string{"metrics"},
			YAML: "k8s_cluster:\n  collection_interval: 30s\n"},
		{ID: "filelog", Label: "File Log", Signals: []string{"logs"},
			YAML: "filelog:\n  include: [/var/log/pods/*/*/*.log]\n  start_at: end\n"},
		{ID: "prometheus", Label: "Prometheus", Signals: []string{"metrics"},
			YAML: "prometheus:\n  config:\n    scrape_configs: []\n"},
		{ID: "jaeger", Label: "Jaeger", Signals: []string{"traces"},
			YAML: "jaeger:\n  protocols:\n    grpc:\n    thrift_http:\n"},
		{ID: "zipkin", Label: "Zipkin", Signals: []string{"traces"},
			YAML: "zipkin:\n"},
	},
	Processors: []component{
		{ID: "batch", Label: "Batch", Signals: []string{"logs", "metrics", "traces"},
			YAML: "batch:\n  timeout: 5s\n"},
		{ID: "memory_limiter", Label: "Memory Limiter", Signals: []string{"logs", "metrics", "traces"},
			YAML: "memory_limiter:\n  check_interval: 1s\n  limit_mib: 512\n"},
		{ID: "resource", Label: "Resource Attributes", Signals: []string{"logs", "metrics", "traces"},
			YAML: "resource:\n  attributes:\n    - key: k8s.namespace.name\n      action: upsert\n"},
		{ID: "k8sattributes", Label: "Kubernetes Attributes", Signals: []string{"logs", "metrics", "traces"},
			YAML: "k8sattributes:\n  passthrough: false\n"},
		{ID: "attributes", Label: "Attributes", Signals: []string{"logs", "metrics", "traces"},
			YAML: "attributes:\n  actions: []\n"},
		{ID: "filter", Label: "Filter", Signals: []string{"logs", "metrics", "traces"},
			YAML: "filter:\n  error_mode: ignore\n"},
		{ID: "transform", Label: "Transform", Signals: []string{"logs", "metrics", "traces"},
			YAML: "transform:\n  error_mode: ignore\n"},
	},
	Connectors: []component{
		{ID: "routing", Label: "Routing", Signals: []string{"logs", "metrics", "traces"},
			YAML: "routing:\n  default_pipelines: []\n  table: []\n"},
		{ID: "forward", Label: "Forward", Signals: []string{"logs", "metrics", "traces"},
			YAML: "forward:\n"},
		{ID: "count", Label: "Count", Signals: []string{"logs", "metrics", "traces"},
			YAML: "count:\n"},
	},
	Exporters: []component{
		{ID: "otlp", Label: "OTLP", Signals: []string{"logs", "metrics", "traces"},
			YAML: "otlp:\n  endpoint: ${env:OTLP_ENDPOINT}\n"},
		{ID: "otlphttp", Label: "OTLP HTTP", Signals: []string{"logs", "metrics", "traces"},
			YAML: "otlphttp:\n  endpoint: ${env:OTLP_HTTP_ENDPOINT}\n"},
		{ID: "prometheus", Label: "Prometheus", Signals: []string{"metrics"},
			YAML: "prometheus:\n  endpoint: 0.0.0.0:8889\n"},
		{ID: "debug", Label: "Debug", Signals: []string{"logs", "metrics", "traces"},
			YAML: "debug:\n  verbosity: basic\n"},
		{ID: "file", Label: "File", Signals: []string{"logs", "metrics", "traces"},
			YAML: "file:\n  path: /tmp/otel-output.json\n"},
	},
	Extensions: []component{
		{ID: "health_check", Label: "Health Check", Signals: []string{"logs", "metrics", "traces"},
			YAML: "health_check:\n  endpoint: 0.0.0.0:13133\n"},
		{ID: "pprof", Label: "pprof", Signals: []string{"logs", "metrics", "traces"},
			YAML: "pprof:\n  endpoint: 0.0.0.0:1777\n"},
		{ID: "zpages", Label: "zPages", Signals: []string{"logs", "metrics", "traces"},
			YAML: "zpages:\n  endpoint: 0.0.0.0:55679\n"},
	},
}
