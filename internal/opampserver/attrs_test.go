package opampserver

import (
	"testing"

	"github.com/open-telemetry/opamp-go/protobufs"
)

func strAttr(key, val string) *protobufs.KeyValue {
	return &protobufs.KeyValue{Key: key, Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: val}}}
}

func TestFlattenAttributes_IdentifyingWinsOverNonIdentifying(t *testing.T) {
	desc := &protobufs.AgentDescription{
		IdentifyingAttributes:    []*protobufs.KeyValue{strAttr("service.name", "from-identifying")},
		NonIdentifyingAttributes: []*protobufs.KeyValue{strAttr("service.name", "from-non-identifying"), strAttr("extra", "value")},
	}
	attrs := flattenAttributes(desc)
	if attrs["service.name"] != "from-identifying" {
		t.Errorf("expected identifying attribute to win, got %q", attrs["service.name"])
	}
	if attrs["extra"] != "value" {
		t.Errorf("expected non-identifying-only attribute to still be present, got %q", attrs["extra"])
	}
}

func TestFlattenAttributes_NilDescription(t *testing.T) {
	attrs := flattenAttributes(nil)
	if attrs == nil || len(attrs) != 0 {
		t.Errorf("expected an empty, non-nil map for a nil description, got %#v", attrs)
	}
}

func TestAnyValueToString(t *testing.T) {
	cases := []struct {
		name string
		v    *protobufs.AnyValue
		want string
	}{
		{"nil", nil, ""},
		{"string", &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "hi"}}, "hi"},
		{"bool true", &protobufs.AnyValue{Value: &protobufs.AnyValue_BoolValue{BoolValue: true}}, "true"},
		{"bool false", &protobufs.AnyValue{Value: &protobufs.AnyValue_BoolValue{BoolValue: false}}, "false"},
		{"int", &protobufs.AnyValue{Value: &protobufs.AnyValue_IntValue{IntValue: 42}}, "42"},
		{"double", &protobufs.AnyValue{Value: &protobufs.AnyValue_DoubleValue{DoubleValue: 3.5}}, "3.5"},
		{"bytes", &protobufs.AnyValue{Value: &protobufs.AnyValue_BytesValue{BytesValue: []byte("raw")}}, "raw"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := anyValueToString(c.v); got != c.want {
				t.Errorf("anyValueToString() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestMetricsPortOf(t *testing.T) {
	cases := []struct {
		name  string
		attrs map[string]string
		want  uint16
	}{
		{"absent", map[string]string{}, 0},
		{"valid", map[string]string{attrSelfMetricsPort: "8888"}, 8888},
		{"zero rejected", map[string]string{attrSelfMetricsPort: "0"}, 0},
		{"out of range rejected", map[string]string{attrSelfMetricsPort: "99999"}, 0},
		{"non-numeric rejected", map[string]string{attrSelfMetricsPort: "not-a-port"}, 0},
		{"negative rejected", map[string]string{attrSelfMetricsPort: "-1"}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := metricsPortOf(c.attrs); got != c.want {
				t.Errorf("metricsPortOf(%v) = %d, want %d", c.attrs, got, c.want)
			}
		})
	}
}
