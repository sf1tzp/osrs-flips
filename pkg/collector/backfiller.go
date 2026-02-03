package collector

import (
	"context"
	"sync"
	"time"

	"osrs-flipping/pkg/logging"
	"osrs-flipping/pkg/osrs"
)

// BackfillerConfig configures the backfill service.
type BackfillerConfig struct {
	BucketSizes   []string      // Bucket sizes to backfill (e.g., ["5m", "1h", "24h"])
	RateLimit     time.Duration // Delay between API calls (default: 100ms)
	BatchSize     int           // Items to process before logging progress (default: 100)
	MaxConcurrent int           // Max concurrent API requests (default: 1)
}

// DefaultBackfillerConfig returns sensible defaults.
func DefaultBackfillerConfig() *BackfillerConfig {
	return &BackfillerConfig{
		BucketSizes:   []string{"5m", "1h", "24h"},
		RateLimit:     100 * time.Millisecond,
		BatchSize:     100,
		MaxConcurrent: 1, // Be nice to the API
	}
}

// Backfiller fetches historical timeseries data and populates price_buckets.
type Backfiller struct {
	client *osrs.Client
	repo   *Repository
	config *BackfillerConfig
	logger *logging.Logger

	mu       sync.Mutex
	running  bool
	stopCh   chan struct{}
	progress BackfillProgress
}

// BackfillProgress tracks backfill status.
type BackfillProgress struct {
	TotalItems      int
	ProcessedItems  int
	CurrentItem     int
	CurrentBucket   string
	BucketsInserted int64
	Errors          int
	StartTime       time.Time
}

// NewBackfiller creates a new Backfiller.
func NewBackfiller(client *osrs.Client, repo *Repository, config *BackfillerConfig, logger *logging.Logger) *Backfiller {
	if config == nil {
		config = DefaultBackfillerConfig()
	}
	return &Backfiller{
		client: client,
		repo:   repo,
		config: config,
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

// Run executes the backfill process. Blocks until complete or stopped.
func (b *Backfiller) Run(ctx context.Context) error {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return nil
	}
	b.running = true
	b.stopCh = make(chan struct{})
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		b.running = false
		b.mu.Unlock()
	}()

	b.progress = BackfillProgress{StartTime: time.Now()}

	// Get list of items to backfill from observations or mapping
	items, err := b.getItemsToBackfill(ctx)
	if err != nil {
		return err
	}
	b.progress.TotalItems = len(items)

	b.logger.WithComponent("backfiller").WithFields(map[string]interface{}{
		"total_items":  len(items),
		"bucket_sizes": b.config.BucketSizes,
	}).Info("starting backfill")

	// Process each bucket size
	for _, bucketSize := range b.config.BucketSizes {
		if err := b.backfillBucketSize(ctx, items, bucketSize); err != nil {
			return err
		}
	}

	elapsed := time.Since(b.progress.StartTime)
	b.logger.WithComponent("backfiller").WithFields(map[string]interface{}{
		"elapsed":          elapsed.String(),
		"items_processed":  b.progress.ProcessedItems,
		"buckets_inserted": b.progress.BucketsInserted,
		"errors":           b.progress.Errors,
	}).Info("backfill complete")

	return nil
}

// Stop signals the backfiller to stop.
func (b *Backfiller) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.running {
		close(b.stopCh)
	}
}

// Progress returns current backfill progress.
func (b *Backfiller) Progress() BackfillProgress {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.progress
}

func (b *Backfiller) getItemsToBackfill(ctx context.Context) ([]int, error) {
	// First try to get items from price_observations (items we're actively tracking)
	items, err := b.repo.GetDistinctItemIDs(ctx)
	if err != nil {
		return nil, err
	}

	// If no observations yet, fetch item mapping from API
	if len(items) == 0 {
		b.logger.WithComponent("backfiller").Info("no observations found, fetching item mapping from API")
		mappings, err := b.client.GetItemMapping(ctx)
		if err != nil {
			return nil, err
		}
		items = make([]int, len(mappings))
		for i, m := range mappings {
			items[i] = m.ID
		}
	}

	return items, nil
}

func (b *Backfiller) backfillBucketSize(ctx context.Context, items []int, bucketSize string) error {
	// Get already backfilled items for this bucket size
	backfilled, err := b.repo.GetBackfilledItems(ctx, bucketSize)
	if err != nil {
		return err
	}

	// Filter to items that need backfilling
	var toBackfill []int
	for _, itemID := range items {
		if !backfilled[itemID] {
			toBackfill = append(toBackfill, itemID)
		}
	}

	b.logger.WithComponent("backfiller").WithFields(map[string]interface{}{
		"bucket_size":      bucketSize,
		"items_to_process": len(toBackfill),
		"already_done":     len(backfilled),
	}).Info("backfilling bucket size")

	for i, itemID := range toBackfill {
		select {
		case <-b.stopCh:
			b.logger.WithComponent("backfiller").Info("backfill stopped by signal")
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		b.mu.Lock()
		b.progress.CurrentItem = itemID
		b.progress.CurrentBucket = bucketSize
		b.mu.Unlock()

		if err := b.backfillItem(ctx, itemID, bucketSize); err != nil {
			b.logger.WithComponent("backfiller").WithError(err).WithFields(map[string]interface{}{
				"item_id":     itemID,
				"bucket_size": bucketSize,
			}).Warn("failed to backfill item")
			b.mu.Lock()
			b.progress.Errors++
			b.mu.Unlock()
			// Continue to next item instead of failing entirely
		}

		b.mu.Lock()
		b.progress.ProcessedItems++
		b.mu.Unlock()

		// Log progress periodically
		if (i+1)%b.config.BatchSize == 0 {
			b.logger.WithComponent("backfiller").WithFields(map[string]interface{}{
				"bucket_size": bucketSize,
				"progress":    i + 1,
				"total":       len(toBackfill),
				"percent":     float64(i+1) / float64(len(toBackfill)) * 100,
			}).Info("backfill progress")
		}

		// Rate limiting
		time.Sleep(b.config.RateLimit)
	}

	return nil
}

func (b *Backfiller) backfillItem(ctx context.Context, itemID int, bucketSize string) error {
	// Fetch timeseries from API
	resp, err := b.client.GetTimeseriesTyped(ctx, itemID, bucketSize)
	if err != nil {
		return err
	}

	if len(resp.Data) == 0 {
		return nil // No data for this item
	}

	// Convert to price buckets
	buckets := make([]PriceBucket, 0, len(resp.Data))
	for _, dp := range resp.Data {
		// Skip empty data points
		if dp.AvgHighPrice == nil && dp.AvgLowPrice == nil {
			continue
		}

		bucket := PriceBucket{
			ItemID:       itemID,
			BucketStart:  time.Unix(dp.Timestamp, 0).UTC(),
			BucketSize:   bucketSize,
			AvgHighPrice: dp.AvgHighPrice,
			AvgLowPrice:  dp.AvgLowPrice,
			Source:       "api",
		}

		// Convert volume to int64 pointers (VolumeDataPoint uses HighPriceVol/LowPriceVol)
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

	// Insert buckets
	inserted, err := b.repo.InsertPriceBuckets(ctx, buckets)
	if err != nil {
		return err
	}

	b.mu.Lock()
	b.progress.BucketsInserted += inserted
	b.mu.Unlock()

	return nil
}
