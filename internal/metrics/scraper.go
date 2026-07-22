package metrics

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/common/expfmt"

	"github.com/anasschbada/opamp-fleet-server/internal/opampserver"
)

// maxScrapeBodyBytes bounds how much of a /metrics response we read. A
// well-behaved OTel Collector's self-telemetry page is a few KB; capping
// far above that still protects the server from a misbehaving or
// compromised collector returning an unbounded stream.
const maxScrapeBodyBytes = 2 << 20 // 2 MiB

// scrapeTimeout bounds each individual HTTP GET. Short, because this runs
// on a fixed interval across potentially many agents and a single hung
// collector must not stall the others.
const scrapeTimeout = 5 * time.Second

// Scraper periodically pulls Prometheus-format self-telemetry from every
// connected agent that has advertised a metrics endpoint (see
// opampserver.ScrapeTarget), and turns the handful of metric names common
// to every OpenTelemetry Collector build into the Snapshot the REST API
// serves to the UI.
type Scraper struct {
	targets  func() []opampserver.ScrapeTarget
	store    *Store
	client   *http.Client
	interval time.Duration
	log      *slog.Logger

	mu    sync.Mutex
	prevs map[string]counterSample // previous raw cumulative counters, for rate calculation
}

type counterSample struct {
	at                  time.Time
	cpuSecondsTotal     float64
	receivedPointsTotal float64
	sentPointsTotal     float64
	failedPointsTotal   float64
}

func NewScraper(targets func() []opampserver.ScrapeTarget, store *Store, interval time.Duration, log *slog.Logger) *Scraper {
	return &Scraper{
		targets:  targets,
		store:    store,
		interval: interval,
		log:      log,
		prevs:    make(map[string]counterSample),
		client: &http.Client{
			Timeout: scrapeTimeout,
			// Never follow redirects: a scrape target is a literal IP we
			// just observed on an authenticated OpAMP connection (see
			// registry.go). Following a redirect could send the request
			// somewhere else entirely, which would defeat that guarantee.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: &http.Transport{
				// Go's default Transport transparently requests and
				// decompresses gzip. The io.LimitReader below already
				// bounds this safely (it wraps the decompressing reader,
				// so it caps decompressed bytes actually read, not just
				// the compressed wire size), but disabling compression
				// outright removes the need to reason about that at all --
				// this is a small, fixed self-telemetry payload with
				// nothing to gain from compression, scraped from a target
				// we don't fully trust.
				DisableCompression: true,
			},
		},
	}
}

// Run scrapes every target once per interval until ctx is cancelled. Call
// it in its own goroutine.
func (s *Scraper) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.scrapeAll(ctx)
		}
	}
}

func (s *Scraper) scrapeAll(ctx context.Context) {
	targets := s.targets()
	live := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		live[t.InstanceUID] = struct{}{}
		if err := s.scrapeOne(ctx, t); err != nil {
			s.log.Debug("metrics scrape failed", "instance_uid", t.InstanceUID, "error", err)
		}
	}
	s.store.Prune(live)
}

func (s *Scraper) scrapeOne(ctx context.Context, target opampserver.ScrapeTarget) error {
	url := fmt.Sprintf("http://%s/metrics", net.JoinHostPort(target.RemoteIP, strconv.Itoa(int(target.Port))))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var parser expfmt.TextParser
	families, err := parser.TextToMetricFamilies(io.LimitReader(resp.Body, maxScrapeBodyBytes))
	if err != nil {
		return fmt.Errorf("parse prometheus text format: %w", err)
	}

	now := time.Now().UTC()
	cur := counterSample{
		at:                  now,
		cpuSecondsTotal:     sumCounter(families, "otelcol_process_cpu_seconds", "otelcol_process_cpu_seconds_total"),
		receivedPointsTotal: sumCounter(families, "otelcol_receiver_accepted_metric_points", "otelcol_receiver_accepted_metric_points_total") + sumCounter(families, "otelcol_receiver_accepted_log_records", "otelcol_receiver_accepted_log_records_total") + sumCounter(families, "otelcol_receiver_accepted_spans", "otelcol_receiver_accepted_spans_total"),
		sentPointsTotal:     sumCounter(families, "otelcol_exporter_sent_metric_points", "otelcol_exporter_sent_metric_points_total") + sumCounter(families, "otelcol_exporter_sent_log_records", "otelcol_exporter_sent_log_records_total") + sumCounter(families, "otelcol_exporter_sent_spans", "otelcol_exporter_sent_spans_total"),
		failedPointsTotal:   sumCounter(families, "otelcol_exporter_send_failed_metric_points", "otelcol_exporter_send_failed_metric_points_total") + sumCounter(families, "otelcol_exporter_send_failed_log_records", "otelcol_exporter_send_failed_log_records_total") + sumCounter(families, "otelcol_exporter_send_failed_spans", "otelcol_exporter_send_failed_spans_total"),
	}
	memRSS := sumGauge(families, "otelcol_process_memory_rss")

	s.mu.Lock()
	prev, hasPrev := s.prevs[target.InstanceUID]
	s.prevs[target.InstanceUID] = cur
	s.mu.Unlock()

	if !hasPrev {
		// First scrape for this agent: nothing to compute a rate against
		// yet. Record the gauge alone so memory shows up immediately, and
		// wait for the next tick for the rate-based series.
		s.store.update(target.InstanceUID, func(snap *Snapshot) {
			snap.UpdatedAt = now
			snap.MemoryRSSBytes = appendCapped(snap.MemoryRSSBytes, Point{Time: now, Value: memRSS})
		})
		return nil
	}

	elapsed := cur.at.Sub(prev.at).Seconds()
	if elapsed <= 0 {
		return nil
	}
	successRatio := 0.0
	if total := cur.sentPointsTotal + cur.failedPointsTotal - prev.sentPointsTotal - prev.failedPointsTotal; total > 0 {
		successRatio = (cur.sentPointsTotal - prev.sentPointsTotal) / total
	}

	s.store.update(target.InstanceUID, func(snap *Snapshot) {
		snap.UpdatedAt = now
		snap.CPUSecondsPerSec = appendCapped(snap.CPUSecondsPerSec, Point{Time: now, Value: rate(cur.cpuSecondsTotal, prev.cpuSecondsTotal, elapsed)})
		snap.MemoryRSSBytes = appendCapped(snap.MemoryRSSBytes, Point{Time: now, Value: memRSS})
		snap.ReceivedPointsRate = appendCapped(snap.ReceivedPointsRate, Point{Time: now, Value: rate(cur.receivedPointsTotal, prev.receivedPointsTotal, elapsed)})
		snap.ExportSuccessRatio = appendCapped(snap.ExportSuccessRatio, Point{Time: now, Value: successRatio})
	})
	return nil
}

// rate computes a non-negative per-second rate from two cumulative counter
// readings. A negative delta means the collector process restarted and the
// counter reset to zero -- reported as 0 rather than a nonsensical negative
// number.
func rate(cur, prev, elapsedSeconds float64) float64 {
	delta := cur - prev
	if delta < 0 {
		return 0
	}
	return delta / elapsedSeconds
}
