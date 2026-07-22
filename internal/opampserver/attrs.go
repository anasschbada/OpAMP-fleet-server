package opampserver

import (
	"strconv"

	"github.com/open-telemetry/opamp-go/protobufs"
)

// Well-known OpenTelemetry semantic-convention attribute keys we look for in
// AgentDescription. Any OTel Collector distribution that runs the opamp
// extension with resourcedetection/k8sattributes processors enabled reports
// these -- there is nothing EDOT-specific or vendor-specific here.
const (
	attrServiceName      = "service.name"
	attrServiceNamespace = "service.namespace"
	attrK8sNamespace     = "k8s.namespace.name"
	attrServiceVersion   = "service.version"
	attrK8sNodeName      = "k8s.node.name"
	attrK8sPodName       = "k8s.pod.name"

	// attrSelfMetricsPort is not an OpenTelemetry semantic convention: it's
	// this project's own opt-in convention a collector config can set (as a
	// non-identifying resource attribute) to advertise the port its own
	// Prometheus self-telemetry endpoint listens on, e.g. "8888". See
	// internal/metrics for how this is used -- only the port is taken from
	// this attribute, never a host, and the fleet server never scrapes
	// anything unless collector config explicitly opts in.
	attrSelfMetricsPort = "opamp.fleetserver.self_metrics_port"
)

// metricsPortOf reads and validates attrSelfMetricsPort from an already-
// flattened attribute map. Returns 0 (meaning "no metrics endpoint
// offered") if absent or invalid -- an unparsable value is never treated as
// an error, since it only disables an optional feature.
func metricsPortOf(attrs map[string]string) uint16 {
	raw := attrs[attrSelfMetricsPort]
	if raw == "" {
		return 0
	}
	port, err := strconv.ParseUint(raw, 10, 16)
	if err != nil || port == 0 {
		return 0
	}
	return uint16(port)
}

// flattenAttributes converts AgentDescription's identifying + non-identifying
// attribute lists into a single map. AnyValue is reduced to its string
// representation for storage/display simplicity: the fleet UI only ever
// renders these as text, and the map is complemented by the well-known,
// strongly-typed fields (Namespace, Version, ...) that read paths use to
// filter/sort. Non-string value kinds (bool/int/double/etc.) are formatted
// as strings rather than dropped, so operators can still see arbitrary
// custom attributes a distro chooses to add.
func flattenAttributes(desc *protobufs.AgentDescription) map[string]string {
	out := map[string]string{}
	if desc == nil {
		return out
	}
	for _, kv := range desc.IdentifyingAttributes {
		out[kv.Key] = anyValueToString(kv.Value)
	}
	for _, kv := range desc.NonIdentifyingAttributes {
		if _, exists := out[kv.Key]; !exists {
			out[kv.Key] = anyValueToString(kv.Value)
		}
	}
	return out
}

func anyValueToString(v *protobufs.AnyValue) string {
	if v == nil {
		return ""
	}
	switch val := v.Value.(type) {
	case *protobufs.AnyValue_StringValue:
		return val.StringValue
	case *protobufs.AnyValue_BoolValue:
		if val.BoolValue {
			return "true"
		}
		return "false"
	case *protobufs.AnyValue_IntValue:
		return strconv.FormatInt(val.IntValue, 10)
	case *protobufs.AnyValue_DoubleValue:
		return strconv.FormatFloat(val.DoubleValue, 'g', -1, 64)
	case *protobufs.AnyValue_BytesValue:
		return string(val.BytesValue)
	default:
		// ArrayValue / KvlistValue: rare for these attribute keys in
		// practice, and not needed by the UI today. Skip rather than
		// building a recursive formatter for a case that doesn't occur.
		return ""
	}
}
