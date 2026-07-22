package metrics

import (
	"strings"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

func parseFamilies(t *testing.T, text string) map[string]*dto.MetricFamily {
	t.Helper()
	var parser expfmt.TextParser
	families, err := parser.TextToMetricFamilies(strings.NewReader(text))
	if err != nil {
		t.Fatalf("parse prometheus text: %v", err)
	}
	return families
}

func TestSumCounter(t *testing.T) {
	text := `
# HELP otelcol_receiver_accepted_metric_points_total accepted points
# TYPE otelcol_receiver_accepted_metric_points_total counter
otelcol_receiver_accepted_metric_points_total{receiver="otlp"} 100
otelcol_receiver_accepted_metric_points_total{receiver="hostmetrics"} 50
`
	families := parseFamilies(t, text)
	got := sumCounter(families, "otelcol_receiver_accepted_metric_points", "otelcol_receiver_accepted_metric_points_total")
	if got != 150 {
		t.Errorf("sumCounter() = %v, want 150 (sum across label sets)", got)
	}
}

func TestSumCounter_FirstMatchingNameWins(t *testing.T) {
	text := `
# TYPE otelcol_process_cpu_seconds_total counter
otelcol_process_cpu_seconds_total 5
`
	families := parseFamilies(t, text)
	// Candidate order matters: the non-"_total" name isn't present, so the
	// "_total" one must be used -- and only once, not double-counted.
	got := sumCounter(families, "otelcol_process_cpu_seconds", "otelcol_process_cpu_seconds_total")
	if got != 5 {
		t.Errorf("sumCounter() = %v, want 5", got)
	}
}

func TestSumCounter_MissingMetricReturnsZero(t *testing.T) {
	families := parseFamilies(t, "# no metrics here\n")
	if got := sumCounter(families, "does_not_exist"); got != 0 {
		t.Errorf("sumCounter() on absent metric = %v, want 0", got)
	}
}

func TestSumGauge(t *testing.T) {
	text := `
# TYPE otelcol_process_memory_rss gauge
otelcol_process_memory_rss 104857600
`
	families := parseFamilies(t, text)
	got := sumGauge(families, "otelcol_process_memory_rss")
	if got != 104857600 {
		t.Errorf("sumGauge() = %v, want 104857600", got)
	}
}

func TestRate(t *testing.T) {
	if got := rate(110, 100, 10); got != 1 {
		t.Errorf("rate(110,100,10) = %v, want 1", got)
	}
	// A counter reset (process restart) must yield 0, not a negative rate.
	if got := rate(5, 100, 10); got != 0 {
		t.Errorf("rate on counter reset = %v, want 0", got)
	}
	if got := rate(100, 100, 10); got != 0 {
		t.Errorf("rate with no change = %v, want 0", got)
	}
}

func TestAppendCapped(t *testing.T) {
	var series []Point
	for i := 0; i < maxHistoryPoints+10; i++ {
		series = appendCapped(series, Point{Value: float64(i)})
	}
	if len(series) != maxHistoryPoints {
		t.Fatalf("len(series) = %d, want %d", len(series), maxHistoryPoints)
	}
	// The oldest points should have been dropped, keeping the most recent.
	if series[len(series)-1].Value != float64(maxHistoryPoints+9) {
		t.Errorf("last point = %v, want the most recently appended value", series[len(series)-1].Value)
	}
}
