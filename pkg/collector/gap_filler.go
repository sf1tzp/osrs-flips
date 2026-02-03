package collector

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"osrs-flipping/pkg/logging"
	"osrs-flipping/pkg/osrs"
)

// RetentionPolicy defines retention limits for each bucket size.
var RetentionPolicy = map[string]time.Duration{
	"5m":  7 * 24 * time.Hour,  // 7 days
	"1h":  365 * 24 * time.Hour, // 1 year
	"24h": 0,                    // forever (no limit)
}

// GapFillerConfig configures the gap filling service.
type GapFillerConfig struct {
	BucketSizes   []string      // Bucket sizes to check (default: ["5m", "1h", "24h"])
	ItemsPerRun   int           // Max items to process per run (default: 150)
	RateLimit     time.Duration // Minimum delay between API calls (default: 100ms)
	MaxConcurrent int           // Max concurrent API requests (default: 1)
}

// DefaultGapFillerConfig returns sensible defaults.
func DefaultGapFillerConfig() *GapFillerConfig {
	return &GapFillerConfig{
		BucketSizes:   []string{"5m", "1h", "24h"},
		ItemsPerRun:   150,
		RateLimit:     100 * time.Millisecond,
		MaxConcurrent: 1,
	}
}

// GapFillerProgress tracks gap filling progress.
type GapFillerProgress struct {
	ItemsScanned    int
	ItemsProcessed  int
	GapsFound       int
	BucketsFilled   int64
	Errors          int
	CurrentItem     int
	CurrentBucket   string
	StartTime       time.Time
}

// GapFiller detects and fills missing price buckets within retention windows.
type GapFiller struct {
	client  *osrs.Client
	repo    *Repository
	config  *GapFillerConfig
	logger  *logging.Logger
	limiter *rate.Limiter

	mu       sync.Mutex
	running  bool
	stopCh   chan struct{}
	progress GapFillerProgress
}

// NewGapFiller creates a new GapFiller.
func NewGapFiller(client *osrs.Client, repo *Repository, config *GapFillerConfig, logger *logging.Logger) *GapFiller {
	if config == nil {
		config = DefaultGapFillerConfig()
	}

	limit := rate.Every(config.RateLimit)

	return &GapFiller{
		client:  client,
		repo:    repo,
		config:  config,
		logger:  logger,
		stopCh:  make(chan struct{}),
		limiter: rate.NewLimiter(limit, 1),
	}
}

// Run executes the gap filling process. Blocks until complete or stopped.
func (g *GapFiller) Run(ctx context.Context) error {
	g.mu.Lock()
	if g.running {
		g.mu.Unlock()
		return nil
	}
	g.running = true
	g.stopCh = make(chan struct{})
	g.mu.Unlock()

	defer func() {
		g.mu.Lock()
		g.running = false
		g.mu.Unlock()
	}()

	g.progress = GapFillerProgress{StartTime: time.Now()}

	for _, bucketSize := range g.config.BucketSizes {
		if err := g.fillGapsForBucketSize(ctx, bucketSize); err != nil {
			return err
		}

		// Check for stop signal between bucket sizes
		select {
		case <-g.stopCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	// Log run completion summary
	elapsed := time.Since(g.progress.StartTime)
	g.logger.WithComponent("gap_filler").WithFields(map[string]interface{}{
		"event":             "gap_fill_run_completed",
		"items_processed":   g.progress.ItemsProcessed,
		"total_gaps_filled": g.progress.BucketsFilled,
		"errors":            g.progress.Errors,
		"duration_ms":       elapsed.Milliseconds(),
	}).Info("gap_fill_run_completed")

	return nil
}

// Stop signals the gap filler to stop.
func (g *GapFiller) Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.running {
		close(g.stopCh)
	}
}

// Progress returns current gap filling progress.
func (g *GapFiller) Progress() GapFillerProgress {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.progress
}

func (g *GapFiller) fillGapsForBucketSize(ctx context.Context, bucketSize string) error {
	retention := RetentionPolicy[bucketSize]
	retentionDays := 0
	if retention > 0 {
		retentionDays = int(retention.Hours() / 24)
	}

	g.logger.WithComponent("gap_filler").WithFields(map[string]interface{}{
		"event":          "gap_scan_started",
		"bucket_size":    bucketSize,
		"retention_days": retentionDays,
	}).Info("gap_scan_started")

	scanStart := time.Now()

	// Get items with gaps, prioritized by recent activity
	itemsWithGaps, err := g.repo.GetItemsWithGaps(ctx, bucketSize, retention, g.config.ItemsPerRun)
	if err != nil {
		return err
	}

	g.mu.Lock()
	g.progress.ItemsScanned += len(itemsWithGaps)
	g.progress.GapsFound += len(itemsWithGaps)
	g.mu.Unlock()

	g.logger.WithComponent("gap_filler").WithFields(map[string]interface{}{
		"event":         "gap_scan_completed",
		"bucket_size":   bucketSize,
		"items_scanned": len(itemsWithGaps),
		"gaps_found":    len(itemsWithGaps),
		"duration_ms":   time.Since(scanStart).Milliseconds(),
	}).Info("gap_scan_completed")

	// Fill gaps for each item
	for _, itemID := range itemsWithGaps {
		select {
		case <-g.stopCh:
			g.logger.WithComponent("gap_filler").Info("gap filler stopped by signal")
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		g.mu.Lock()
		g.progress.CurrentItem = itemID
		g.progress.CurrentBucket = bucketSize
		g.mu.Unlock()

		if err := g.fillGapsForItem(ctx, itemID, bucketSize, retention); err != nil {
			g.logger.WithComponent("gap_filler").WithFields(map[string]interface{}{
				"event":       "gap_fill_failed",
				"item_id":     itemID,
				"bucket_size": bucketSize,
				"error":       err.Error(),
			}).Warn("gap_fill_failed")

			g.mu.Lock()
			g.progress.Errors++
			g.mu.Unlock()
			// Continue to next item
		}

		g.mu.Lock()
		g.progress.ItemsProcessed++
		g.mu.Unlock()
	}

	return nil
}

func (g *GapFiller) fillGapsForItem(ctx context.Context, itemID int, bucketSize string, retention time.Duration) error {
	// Wait for rate limiter
	if err := g.limiter.Wait(ctx); err != nil {
		return err
	}

	g.logger.WithComponent("gap_filler").WithFields(map[string]interface{}{
		"event":       "gap_fill_started",
		"item_id":     itemID,
		"bucket_size": bucketSize,
	}).Debug("gap_fill_started")

	// Fetch timeseries from API
	resp, err := g.client.GetTimeseriesTyped(ctx, itemID, bucketSize)
	if err != nil {
		return err
	}

	if len(resp.Data) == 0 {
		return nil
	}

	// Calculate retention cutoff
	var cutoff time.Time
	if retention > 0 {
		cutoff = time.Now().UTC().Add(-retention)
	}

	// Convert to price buckets, filtering by retention
	buckets := make([]PriceBucket, 0, len(resp.Data))
	skipped := 0
	for _, dp := range resp.Data {
		bucketTime := time.Unix(dp.Timestamp, 0).UTC()

		// Skip data outside retention window
		if retention > 0 && bucketTime.Before(cutoff) {
			skipped++
			continue
		}

		// Skip empty data points
		if dp.AvgHighPrice == nil && dp.AvgLowPrice == nil {
			continue
		}

		bucket := PriceBucket{
			ItemID:       itemID,
			BucketStart:  bucketTime,
			BucketSize:   bucketSize,
			AvgHighPrice: dp.AvgHighPrice,
			AvgLowPrice:  dp.AvgLowPrice,
			Source:       "api",
		}

		if dp.HighPriceVol != nil {
			v := int64(*dp.HighPriceVol)
			bucket.HighPriceVolume = &v
		}
		if dp.LowPriceVol != nil {
			v := int64(*dp.LowPriceVol)
			bucket.LowPriceVolume = &v
		}

		buckets = append(buckets, bucket)
	}

	if skipped > 0 {
		g.logger.WithComponent("gap_filler").WithFields(map[string]interface{}{
			"event":       "gap_fill_skipped",
			"item_id":     itemID,
			"bucket_size": bucketSize,
			"reason":      "outside_retention",
			"count":       skipped,
		}).Debug("gap_fill_skipped")
	}

	if len(buckets) == 0 {
		return nil
	}

	// Insert buckets (upsert handles conflicts)
	inserted, err := g.repo.InsertPriceBuckets(ctx, buckets)
	if err != nil {
		return err
	}

	g.mu.Lock()
	g.progress.BucketsFilled += inserted
	g.mu.Unlock()

	g.logger.WithComponent("gap_filler").WithFields(map[string]interface{}{
		"event":            "gap_fill_completed",
		"item_id":          itemID,
		"bucket_size":      bucketSize,
		"buckets_inserted": inserted,
	}).Debug("gap_fill_completed")

	return nil
}
