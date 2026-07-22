package metrics

import (
	dto "github.com/prometheus/client_model/go"
)

// sumCounter adds up every label combination of the first candidate metric
// name that's actually present in families. Collector builds differ on
// whether counter names carry a "_total" suffix (an OpenTelemetry semantic
// convention change over time), so callers pass both spellings; only the
// first match is used, to avoid double-counting if a family somehow existed
// under both names.
func sumCounter(families map[string]*dto.MetricFamily, candidateNames ...string) float64 {
	for _, name := range candidateNames {
		fam, ok := families[name]
		if !ok {
			continue
		}
		var total float64
		for _, m := range fam.GetMetric() {
			if c := m.GetCounter(); c != nil {
				total += c.GetValue()
			}
		}
		return total
	}
	return 0
}

func sumGauge(families map[string]*dto.MetricFamily, candidateNames ...string) float64 {
	for _, name := range candidateNames {
		fam, ok := families[name]
		if !ok {
			continue
		}
		var total float64
		for _, m := range fam.GetMetric() {
			if g := m.GetGauge(); g != nil {
				total += g.GetValue()
			}
		}
		return total
	}
	return 0
}
