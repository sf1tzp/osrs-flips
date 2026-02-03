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
	skipItemSync      = flag.Bool("skip-item-sync", false, "Skip initial item metadata sync from API")
	skipBackfill      = flag.Bool("skip-backfill", false, "Skip background sync (run poller only)")
	syncInterval      = flag.Duration("sync-interval", 30*time.Minute, "Background sync interval")
	syncItemsPerCycle = flag.Int("sync-items-per-cycle", 100, "Max items to sync per bucket per cycle")
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

	// Sync item metadata from API (unless skipped)
	if !*skipItemSync {
		itemSyncerConfig := collector.DefaultItemSyncerConfig()
		itemSyncerConfig.SyncInterval = 0 // Disable periodic sync for now; just sync on start
		itemSyncer := collector.NewItemSyncer(osrsClient, repo, itemSyncerConfig, logger)

		logger.WithComponent("collector").Info("syncing item metadata from API")
		syncCtx, syncCancel := context.WithTimeout(context.Background(), 60*time.Second)
		if err := itemSyncer.Start(syncCtx); err != nil {
			logger.WithComponent("collector").WithError(err).Warn("item sync failed, continuing without item metadata")
		}
		syncCancel()
	}

	// Set up graceful shutdown
	runCtx, runCancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.WithComponent("collector").Info("shutdown signal received, gracefully stopping...")
		runCancel()
	}()

	// Run combined mode: Poller + BackgroundSync concurrently
	runCombinedMode(runCtx, osrsClient, repo, logger)

	// Close database connection
	db.Close()

	logger.WithComponent("collector").Info("collector shutdown complete")
}

func runCombinedMode(ctx context.Context, osrsClient *osrs.Client, repo *collector.Repository, logger *logging.Logger) {
	// Configure poller
	pollerConfig := collector.DefaultPollerConfig()
	if intervalStr := os.Getenv("POLL_INTERVAL_SECONDS"); intervalStr != "" {
		if interval, err := time.ParseDuration(intervalStr + "s"); err == nil {
			pollerConfig.Interval = interval
		}
	}

	// Configure background sync
	syncConfig := collector.DefaultBackgroundSyncConfig()
	syncConfig.RunInterval = *syncInterval
	syncConfig.ItemsPerCycle = *syncItemsPerCycle

	// Create components
	poller := collector.NewPoller(osrsClient, repo, pollerConfig, logger)
	var backgroundSync *collector.BackgroundSync
	if !*skipBackfill {
		backgroundSync = collector.NewBackgroundSync(osrsClient, repo, syncConfig, logger, nil)
	}

	// Start poller
	poller.Start()
	logger.WithComponent("collector").WithFields(map[string]interface{}{
		"poll_interval": pollerConfig.Interval.String(),
	}).Info("poller started")

	// Start background sync if enabled
	if backgroundSync != nil {
		backgroundSync.Start()
		logger.WithComponent("collector").WithFields(map[string]interface{}{
			"sync_interval":   syncConfig.RunInterval.String(),
			"items_per_cycle": syncConfig.ItemsPerCycle,
			"bucket_sizes":    syncConfig.BucketSizes,
		}).Info("background sync started")
	}

	logger.WithComponent("collector").Info("combined mode fully initialized")

	// Wait for shutdown signal
	<-ctx.Done()

	// Graceful shutdown
	logger.WithComponent("collector").Info("stopping components...")

	poller.Stop()
	logger.WithComponent("collector").Info("poller stopped")

	if backgroundSync != nil {
		backgroundSync.Stop()
		logger.WithComponent("collector").Info("background sync stopped")
	}
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
