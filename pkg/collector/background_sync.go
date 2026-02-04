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
// Policies defined on database tables in: migrations/004_split_buckets_and_retention.up.sql
var RetentionPolicy = map[string]time.Duration{
	"5m":  7 * 24 * time.Hour,   // 7 days
	"1h":  365 * 24 * time.Hour, // 1 year
	"24h": 0,                    // forever (no limit)
}

// BackgroundSyncConfig configures the background sync service.
type BackgroundSyncConfig struct {
	BucketSizes   []string      // Bucket sizes to sync (default: ["5m", "1h", "24h"])
	RunInterval   time.Duration // How often to run a full sync cycle (default: 30m)
	ItemsPerCycle int           // Max items to process per bucket per cycle (default: 100)
	RateLimit     time.Duration // Minimum delay between API calls (default: 100ms)
}

// DefaultBackgroundSyncConfig returns sensible defaults.
func DefaultBackgroundSyncConfig() *BackgroundSyncConfig {
	return &BackgroundSyncConfig{
		BucketSizes:   []string{"5m", "1h", "24h"},
		RunInterval:   30 * time.Minute,
		ItemsPerCycle: 100,
		RateLimit:     100 * time.Millisecond,
	}
}

// BackgroundSyncProgress tracks sync progress.
type BackgroundSyncProgress struct {
	CyclesCompleted int
	ItemsSynced     int64
	BucketsFilled   int64
	Errors          int
	LastCycleStart  time.Time
	LastCycleEnd    time.Time
}

// BackgroundSync continuously syncs historical price data in the background.
// It replaces both Backfiller and GapFiller with unified logic that handles
// both new items and gap repair.
type BackgroundSync struct {
	client  *osrs.Client
	repo    *Repository
	config  *BackgroundSyncConfig
	logger  *logging.Logger
	limiter *rate.Limiter

	mu       sync.Mutex
	running  bool
	stopCh   chan struct{}
	doneCh   chan struct{}
	progress BackgroundSyncProgress
}

// NewBackgroundSync creates a new BackgroundSync.
// If limiter is nil, an internal rate limiter is created from config.RateLimit.
func NewBackgroundSync(client *osrs.Client, repo *Repository, config *BackgroundSyncConfig, logger *logging.Logger, limiter *rate.Limiter) *BackgroundSync {
	if config == nil {
		config = DefaultBackgroundSyncConfig()
	}

	// Create noop logger if not provided (for testing)
	if logger == nil {
		logger = logging.NewLogger("error", "json") // Minimal logging
	}

	// Create internal rate limiter if not provided
	if limiter == nil {
		limiter = rate.NewLimiter(rate.Every(config.RateLimit), 1)
	}

	return &BackgroundSync{
		client:  client,
		repo:    repo,
		config:  config,
		logger:  logger,
		limiter: limiter,
	}
}

// Start begins the background sync loop in a goroutine.
// Non-blocking - returns immediately.
func (b *BackgroundSync) Start() {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return
	}
	b.running = true
	b.stopCh = make(chan struct{})
	b.doneCh = make(chan struct{})
	b.mu.Unlock()

	go b.run()
}

// Stop signals the background sync to stop and waits for it to finish.
func (b *BackgroundSync) Stop() {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return
	}
	b.mu.Unlock()

	close(b.stopCh)
	<-b.doneCh // Wait for run() to finish
}

// Progress returns current sync progress.
func (b *BackgroundSync) Progress() BackgroundSyncProgress {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.progress
}

// Running returns whether the sync is currently running.
func (b *BackgroundSync) Running() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

func (b *BackgroundSync) run() {
	defer func() {
		b.mu.Lock()
		b.running = false
		b.mu.Unlock()
		close(b.doneCh)
	}()

	b.logger.WithComponent("background_sync").WithFields(map[string]interface{}{
		"bucket_sizes":    b.config.BucketSizes,
		"run_interval":    b.config.RunInterval.String(),
		"items_per_cycle": b.config.ItemsPerCycle,
	}).Info("starting background sync")

	// Run immediately on start
	b.runCycle()

	ticker := time.NewTicker(b.config.RunInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			b.logger.WithComponent("background_sync").Info("background sync stopped")
			return
		case <-ticker.C:
			b.runCycle()
		}
	}
}

func (b *BackgroundSync) runCycle() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Allow cancellation via stopCh
	go func() {
		select {
		case <-b.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	b.mu.Lock()
	b.progress.LastCycleStart = time.Now()
	b.mu.Unlock()

	cycleItemsSynced := int64(0)
	cycleBucketsFilled := int64(0)
	cycleErrors := 0

	for _, bucketSize := range b.config.BucketSizes {
		select {
		case <-ctx.Done():
			return
		default:
		}

		itemsSynced, bucketsFilled, errors := b.syncBucketSize(ctx, bucketSize)
		cycleItemsSynced += itemsSynced
		cycleBucketsFilled += bucketsFilled
		cycleErrors += errors
	}

	b.mu.Lock()
	b.progress.CyclesCompleted++
	b.progress.ItemsSynced += cycleItemsSynced
	b.progress.BucketsFilled += cycleBucketsFilled
	b.progress.Errors += cycleErrors
	b.progress.LastCycleEnd = time.Now()
	cycleDuration := b.progress.LastCycleEnd.Sub(b.progress.LastCycleStart)
	cycleNum := b.progress.CyclesCompleted
	b.mu.Unlock()

	b.logger.WithComponent("background_sync").WithFields(map[string]interface{}{
		"cycle":          cycleNum,
		"items_synced":   cycleItemsSynced,
		"buckets_filled": cycleBucketsFilled,
		"errors":         cycleErrors,
		"duration":       cycleDuration.String(),
	}).Info("sync cycle completed")
}

func (b *BackgroundSync) syncBucketSize(ctx context.Context, bucketSize string) (itemsSynced int64, bucketsFilled int64, errors int) {
	retention := RetentionPolicy[bucketSize]

	// Get items that need sync
	items, err := b.repo.GetItemsNeedingSync(ctx, bucketSize, retention, b.config.ItemsPerCycle)
	if err != nil {
		b.logger.WithComponent("background_sync").WithError(err).WithField("bucket_size", bucketSize).Error("failed to get items needing sync")
		return 0, 0, 1
	}

	if len(items) == 0 {
		b.logger.WithComponent("background_sync").WithField("bucket_size", bucketSize).Debug("no items need sync")
		return 0, 0, 0
	}

	b.logger.WithComponent("background_sync").WithFields(map[string]interface{}{
		"bucket_size": bucketSize,
		"items_count": len(items),
	}).Debug("syncing items")

	for _, itemID := range items {
		select {
		case <-ctx.Done():
			return itemsSynced, bucketsFilled, errors
		default:
		}

		filled, err := b.syncItem(ctx, itemID, bucketSize, retention)
		if err != nil {
			b.logger.WithComponent("background_sync").WithError(err).WithFields(map[string]interface{}{
				"item_id":     itemID,
				"bucket_size": bucketSize,
			}).Warn("failed to sync item")
			errors++
			continue
		}

		itemsSynced++
		bucketsFilled += filled
	}

	return itemsSynced, bucketsFilled, errors
}

func (b *BackgroundSync) syncItem(ctx context.Context, itemID int, bucketSize string, retention time.Duration) (int64, error) {
	// Wait for rate limiter
	if err := b.limiter.Wait(ctx); err != nil {
		return 0, err
	}

	// Fetch timeseries from API
	resp, err := b.client.GetTimeseriesTyped(ctx, itemID, bucketSize)
	if err != nil {
		return 0, err
	}

	if len(resp.Data) == 0 {
		return 0, nil
	}

	// Calculate retention cutoff
	var cutoff time.Time
	if retention > 0 {
		cutoff = time.Now().UTC().Add(-retention)
	}

	// Convert to price buckets, filtering by retention
	buckets := make([]PriceBucket, 0, len(resp.Data))
	for _, dp := range resp.Data {
		bucketTime := time.Unix(dp.Timestamp, 0).UTC()

		// Skip data outside retention window
		if retention > 0 && bucketTime.Before(cutoff) {
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

	if len(buckets) == 0 {
		return 0, nil
	}

	// Insert buckets (upsert handles conflicts)
	inserted, err := b.repo.InsertPriceBuckets(ctx, buckets)
	if err != nil {
		return 0, err
	}

	return inserted, nil
}
