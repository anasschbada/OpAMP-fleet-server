// Command opamp-server runs the OpAMP fleet management server: an OpAMP
// protocol listener for OpenTelemetry Collector agents, and a REST API for
// the fleet UI. See docs/RBAC.md for the Kubernetes permission model (none,
// by default) and README.md for how to run it.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/anasschbada/opamp-fleet-server/internal/api"
	"github.com/anasschbada/opamp-fleet-server/internal/auth"
	"github.com/anasschbada/opamp-fleet-server/internal/config"
	"github.com/anasschbada/opamp-fleet-server/internal/metrics"
	"github.com/anasschbada/opamp-fleet-server/internal/opampserver"
	"github.com/anasschbada/opamp-fleet-server/internal/store"
)

const (
	metricsScrapeInterval    = 15 * time.Second
	authTokenReloadInterval  = 60 * time.Second
	authLimiterCleanupPeriod = time.Minute
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal startup error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(nil)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := newLogger(cfg.LogLevel)
	slog.SetDefault(log)

	// Two SEPARATE token pools: a collector must never also be able to
	// authenticate to the REST API and push config to other agents. See
	// internal/config.Config's field comment for the full rationale.
	agentTokens, err := auth.NewTokenVerifier(cfg.AgentAuthTokensFile)
	if err != nil {
		return fmt.Errorf("load agent auth tokens: %w", err)
	}
	apiTokens, err := auth.NewTokenVerifier(cfg.APIAuthTokensFile)
	if err != nil {
		return fmt.Errorf("load api auth tokens: %w", err)
	}

	st, err := store.NewSQLiteStore(filepath.Join(cfg.DataDir, "fleet.db"))
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	var tlsConfig *tls.Config
	if cfg.TLSCertFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return fmt.Errorf("load TLS keypair: %w", err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
	} else {
		log.Warn("TLS is not configured (TLS_CERT_FILE/TLS_KEY_FILE unset): OpAMP and REST traffic will be plaintext. " +
			"Only acceptable behind a TLS-terminating mesh/ingress inside a trusted network boundary.")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	opampHandler := opampserver.NewHandler(st, agentTokens, log)
	metricsStore := metrics.NewStore()

	agentTokens.StartAutoReload(ctx, authTokenReloadInterval, log)
	apiTokens.StartAutoReload(ctx, authTokenReloadInterval, log)
	opampHandler.StartAuthLimiterCleanup(ctx, authLimiterCleanupPeriod)
	go opampserver.RunStaleSweeper(ctx, st, cfg.StaleAfter, cfg.DisconnectedAfter, log)

	scraper := metrics.NewScraper(opampHandler.ScrapeTargets, metricsStore, metricsScrapeInterval, log)
	go scraper.Run(ctx)

	opampHTTPServer, err := opampserver.NewHTTPServer(opampHandler, cfg.OpAMPListenAddr, tlsConfig, log)
	if err != nil {
		return fmt.Errorf("build opamp http server: %w", err)
	}
	opampErrCh := serve(opampHTTPServer, tlsConfig != nil)
	log.Info("opamp listener started", "addr", cfg.OpAMPListenAddr, "tls", tlsConfig != nil)

	restHandler := api.NewHandler(ctx, st, opampHandler, metricsStore, apiTokens, log)
	restServer := &http.Server{
		Addr:              cfg.APIListenAddr,
		Handler:           restHandler,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: 10 * time.Second, // mitigates slow-header-style DoS against the REST listener
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	restErrCh := serve(restServer, tlsConfig != nil)
	log.Info("rest api listener started", "addr", cfg.APIListenAddr, "tls", tlsConfig != nil)

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-opampErrCh:
		if err != nil {
			return fmt.Errorf("opamp listener failed: %w", err)
		}
	case err := <-restErrCh:
		if err != nil {
			return fmt.Errorf("rest api listener failed: %w", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := restServer.Shutdown(shutdownCtx); err != nil {
		log.Error("rest api shutdown error", "error", err)
	}
	if err := opampHTTPServer.Shutdown(shutdownCtx); err != nil {
		log.Error("opamp listener shutdown error", "error", err)
	}
	return nil
}

// serve starts srv in the background and reports any error other than the
// expected "server closed" one on the returned channel (which is closed
// once the server stops, whether cleanly or not).
func serve(srv *http.Server, useTLS bool) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		var err error
		if useTLS {
			err = srv.ListenAndServeTLS("", "")
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()
	return errCh
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	// JSON output: this server always runs as a container, where stdout is
	// scraped by a log pipeline that expects structured lines, not a
	// human terminal.
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
