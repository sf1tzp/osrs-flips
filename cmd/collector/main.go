package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"osrs-flipping/pkg/database"
	"osrs-flipping/pkg/logging"
)

const VERSION = "0.0.1"

func main() {
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
		"total_conns":   stats.TotalConns(),
		"idle_conns":    stats.IdleConns(),
		"acquired":      stats.AcquiredConns(),
		"max_conns":     stats.MaxConns(),
	}).Info("database pool initialized")

	// TODO: Initialize and start collector services
	// - Polling service for /latest endpoint
	// - Backfill service for /timeseries endpoint
	// - Gap filling service

	logger.WithComponent("collector").Info("collector fully initialized, waiting for shutdown signal")

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	logger.WithComponent("collector").Info("shutdown signal received, gracefully stopping...")

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
