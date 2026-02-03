package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"osrs-flipping/pkg/collector"
	"osrs-flipping/pkg/database"
	"osrs-flipping/pkg/logging"
	"osrs-flipping/pkg/osrs"
)

const VERSION = "0.0.1"

var (
	backfillMode = flag.Bool("backfill", false, "Run historical backfill instead of continuous polling")
	backfillOnly = flag.String("backfill-bucket", "", "Backfill only specific bucket size (5m, 1h, 24h)")
)

func main() {
	flag.Parse()
	// Initialize logger (default to info/json for now)
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logFormat := os.Getenv("LOG_FORMAT")
	if logFormat == "" {
		logFormat = "json"
	}
	logger := logging.NewLogger(logLevel, logFormat)

	logger.WithComponent("collector").WithField("version", VERSION).Info("starting price data collector")

	// Load database configuration from environment
	dbConfig, err := database.ConfigFromEnv()
	if err != nil {
		logger.WithComponent("collector").WithError(err).Fatal("failed to load database configuration")
	}

	// Get OSRS API user agent (required by RuneScape Wiki API)
	userAgent := os.Getenv("OSRS_API_USER_AGENT")
	if userAgent == "" {
		logger.WithComponent("collector").Fatal("OSRS_API_USER_AGENT environment variable is required")
	}

	// Connect to database
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	db, err := database.Connect(ctx, dbConfig)
	cancel()
	if err != nil {
		logger.WithComponent("collector").WithError(err).Fatal("failed to connect to database")
	}
	defer db.Close()

	logger.WithComponent("collector").Info("connected to database")

	// Run migrations
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "migrations"
	}

	logger.WithComponent("collector").WithField("path", migrationsPath).Info("running database migrations")
	if err := database.MigrateUp(dbConfig.DatabaseURL, migrationsPath); err != nil {
		logger.WithComponent("collector").WithError(err).Fatal("failed to run migrations")
	}

	version, dirty, err := database.MigrateVersion(dbConfig.DatabaseURL, migrationsPath)
	if err != nil {
		logger.WithComponent("collector").WithError(err).Warn("failed to get migration version")
	} else {
		logger.WithComponent("collector").WithFields(map[string]interface{}{
			"version": version,
			"dirty":   dirty,
		}).Info("migrations complete")
	}

	// Log connection pool stats
	stats := db.Stats()
	logger.WithComponent("collector").WithFields(map[string]interface{}{
		"total_conns": stats.TotalConns(),
		"idle_conns":  stats.IdleConns(),
		"acquired":    stats.AcquiredConns(),
		"max_conns":   stats.MaxConns(),
	}).Info("database pool initialized")

	// Initialize OSRS API client
	osrsClient := osrs.NewClient(userAgent)

	// Initialize repository
	repo := collector.NewRepository(db.Pool)

	// Set up graceful shutdown
	runCtx, runCancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.WithComponent("collector").Info("shutdown signal received, gracefully stopping...")
		runCancel()
	}()

	if *backfillMode {
		// Run backfill mode
		backfillerConfig := collector.DefaultBackfillerConfig()
		if *backfillOnly != "" {
			backfillerConfig.BucketSizes = []string{*backfillOnly}
		}

		backfiller := collector.NewBackfiller(osrsClient, repo, backfillerConfig, logger)

		logger.WithComponent("collector").WithFields(map[string]interface{}{
			"bucket_sizes": backfillerConfig.BucketSizes,
			"rate_limit":   backfillerConfig.RateLimit.String(),
		}).Info("starting backfill mode")

		if err := backfiller.Run(runCtx); err != nil && err != context.Canceled {
			logger.WithComponent("collector").WithError(err).Error("backfill failed")
		}
	} else {
		// Run continuous polling mode
		pollerConfig := collector.DefaultPollerConfig()
		if intervalStr := os.Getenv("POLL_INTERVAL_SECONDS"); intervalStr != "" {
			if interval, err := time.ParseDuration(intervalStr + "s"); err == nil {
				pollerConfig.Interval = interval
			}
		}

		poller := collector.NewPoller(osrsClient, repo, pollerConfig, logger)
		poller.Start()

		logger.WithComponent("collector").WithFields(map[string]interface{}{
			"poll_interval": pollerConfig.Interval.String(),
		}).Info("collector fully initialized, polling started")

		// Wait for context cancellation
		<-runCtx.Done()

		// Stop poller
		poller.Stop()
	}

	// Close database connection
	db.Close()

	logger.WithComponent("collector").Info("collector shutdown complete")
}

func init() {
	// Ensure we exit with proper code on panic
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "panic: %v\n", r)
			os.Exit(1)
		}
	}()
}
