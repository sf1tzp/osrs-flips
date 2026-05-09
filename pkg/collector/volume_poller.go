package collector

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"osrs-flipping/pkg/logging"
	"osrs-flipping/pkg/osrs"
)

// VolumePollerConfig configures the volume polling service.
type VolumePollerConfig struct {
	PollInterval time.Duration // How often to poll (default: 5m)
	RateLimit    time.Duration // Delay between API calls (default: 100ms)
	RetryDelay   time.Duration // Delay between retries on failure (default: 10s)
	MaxRetries   int           // Max consecutive failures before backing off (default: 5)
	BackoffMax   time.Duration // Maximum backoff duration (default: 5m)
}

// DefaultVolumePollerConfig returns sensible defaults.
func DefaultVolumePollerConfig() *VolumePollerConfig {
	return &VolumePollerConfig{
		PollInterval: 5 * time.Minute,
		RateLimit:    100 * time.Millisecond,
		RetryDelay:   10 * time.Second,
		MaxRetries:   5,
		BackoffMax:   5 * time.Minute,
	}
}

// VolumePollerProgress tracks polling progress.
type VolumePollerProgress struct {
	CyclesCompleted int
	ItemsPolled     int64
	BucketsFilled   int64
	Errors          int
	LastPollStart   time.Time
	LastPollEnd     time.Time
}

// VolumePoller polls 5m timeseries data for items with poll_volume=true.
// This enables volume-based signal detection for high-priority items.
type VolumePoller struct {
	client  *osrs.Client
	repo    *Repository
	config  *VolumePollerConfig
	logger  *logging.Logger
	limiter *rate.Limiter

	mu               sync.Mutex
	running          bool
	stopCh           chan struct{}
	doneCh           chan struct{}
	progress         VolumePollerProgress
	consecutiveFails int
}

// NewVolumePoller creates a new VolumePoller.
// If limiter is nil, an internal rate limiter is created from config.RateLimit.
func NewVolumePoller(client *osrs.Client, repo *Repository, config *VolumePollerConfig, logger *logging.Logger, limiter *rate.Limiter) *VolumePoller {
	if config == nil {
		config = DefaultVolumePollerConfig()
	}

	if logger == nil {
		logger = logging.NewLogger("error", "json")
	}

	if limiter == nil {
		limiter = rate.NewLimiter(rate.Every(config.RateLimit), 1)
	}

	return &VolumePoller{
		client:  client,
		repo:    repo,
		config:  config,
		logger:  logger,
		limiter: limiter,
	}
}

// Start begins the polling loop in a goroutine.
// Non-blocking - returns immediately.
func (v *VolumePoller) Start() {
	v.mu.Lock()
	if v.running {
		v.mu.Unlock()
		return
	}
	v.running = true
	v.stopCh = make(chan struct{})
	v.doneCh = make(chan struct{})
	v.mu.Unlock()

	go v.run()
}

// Stop signals the poller to stop and waits for it to finish.
func (v *VolumePoller) Stop() {
	v.mu.Lock()
	if !v.running {
		v.mu.Unlock()
		return
	}
	v.mu.Unlock()

	close(v.stopCh)
	<-v.doneCh // Wait for run() to finish
}

// Progress returns current polling progress.
func (v *VolumePoller) Progress() VolumePollerProgress {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.progress
}

// Running returns whether the poller is currently running.
func (v *VolumePoller) Running() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.running
}

func (v *VolumePoller) run() {
	defer func() {
		v.mu.Lock()
		v.running = false
		v.mu.Unlock()
		close(v.doneCh)
	}()

	v.logger.WithComponent("volume_poller").WithFields(map[string]interface{}{
		"poll_interval": v.config.PollInterval.String(),
		"rate_limit":    v.config.RateLimit.String(),
	}).Info("starting volume poller")

	// Run immediately on start
	v.poll()

	ticker := time.NewTicker(v.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-v.stopCh:
			v.logger.WithComponent("volume_poller").Info("volume poller stopped")
			return
		case <-ticker.C:
			v.poll()
		}
	}
}

func (v *VolumePoller) poll() {
	// Guard against nil dependencies (for testing)
	if v.repo == nil || v.client == nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Allow cancellation via stopCh
	go func() {
		select {
		case <-v.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	v.mu.Lock()
	v.progress.LastPollStart = time.Now()
	v.mu.Unlock()

	// Get items to poll
	items, err := v.repo.GetItemsToPollVolume(ctx)
	if err != nil {
		v.handleError(err)
		return
	}

	if len(items) == 0 {
		v.logger.WithComponent("volume_poller").Debug("no items with poll_volume=true")
		v.consecutiveFails = 0
		return
	}

	v.logger.WithComponent("volume_poller").WithField("items_count", len(items)).Debug("polling volume data")

	var itemsPolled int64
	var bucketsFilled int64
	var errors int

	for _, itemID := range items {
		select {
		case <-ctx.Done():
			return
		default:
		}

		filled, err := v.pollItem(ctx, itemID)
		if err != nil {
			v.logger.WithComponent("volume_poller").WithError(err).WithField("item_id", itemID).Warn("failed to poll item")
			errors++
			continue
		}

		itemsPolled++
		bucketsFilled += filled
	}

	// Update progress
	v.mu.Lock()
	v.progress.CyclesCompleted++
	v.progress.ItemsPolled += itemsPolled
	v.progress.BucketsFilled += bucketsFilled
	v.progress.Errors += errors
	v.progress.LastPollEnd = time.Now()
	duration := v.progress.LastPollEnd.Sub(v.progress.LastPollStart)
	cycleNum := v.progress.CyclesCompleted
	v.mu.Unlock()

	// Reset failure counter on successful cycle
	v.consecutiveFails = 0

	v.logger.WithComponent("volume_poller").WithFields(map[string]interface{}{
		"cycle":          cycleNum,
		"items_polled":   itemsPolled,
		"buckets_filled": bucketsFilled,
		"errors":         errors,
		"duration":       duration.String(),
	}).Info("volume poll completed")
}

func (v *VolumePoller) pollItem(ctx context.Context, itemID int) (int64, error) {
	// Wait for rate limiter
	if err := v.limiter.Wait(ctx); err != nil {
		return 0, err
	}

	// Fetch 5m timeseries from API
	resp, err := v.client.GetTimeseriesTyped(ctx, itemID, "5m")
	if err != nil {
		return 0, err
	}

	if len(resp.Data) == 0 {
		return 0, nil
	}

	// Calculate retention cutoff (7 days for 5m buckets)
	retention := RetentionPolicy["5m"]
	var cutoff time.Time
	if retention > 0 {
		cutoff = time.Now().UTC().Add(-retention)
	}

	// Convert to price buckets
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
			BucketSize:   "5m",
			AvgHighPrice: dp.AvgHighPrice,
			AvgLowPrice:  dp.AvgLowPrice,
			Source:       "api",
		}

		if dp.HighPriceVol != nil {
			vol := int64(*dp.HighPriceVol)
			bucket.HighPriceVolume = &vol
		}
		if dp.LowPriceVol != nil {
			vol := int64(*dp.LowPriceVol)
			bucket.LowPriceVolume = &vol
		}

		buckets = append(buckets, bucket)
	}

	if len(buckets) == 0 {
		return 0, nil
	}

	// Insert buckets (upsert handles conflicts)
	inserted, err := v.repo.InsertPriceBuckets(ctx, buckets)
	if err != nil {
		return 0, err
	}

	return inserted, nil
}

func (v *VolumePoller) handleError(err error) {
	v.consecutiveFails++

	v.logger.WithComponent("volume_poller").WithError(err).WithField("consecutive_fails", v.consecutiveFails).Error("poll failed")

	// Implement exponential backoff if too many failures
	if v.consecutiveFails >= v.config.MaxRetries {
		backoff := time.Duration(v.consecutiveFails-v.config.MaxRetries+1) * v.config.RetryDelay
		if backoff > v.config.BackoffMax {
			backoff = v.config.BackoffMax
		}
		v.logger.WithComponent("volume_poller").WithField("backoff", backoff).Warn("backing off due to repeated failures")
		time.Sleep(backoff)
	}
}

// Stats returns current poller statistics.
func (v *VolumePoller) Stats() map[string]interface{} {
	v.mu.Lock()
	defer v.mu.Unlock()
	return map[string]interface{}{
		"running":           v.running,
		"consecutive_fails": v.consecutiveFails,
		"cycles_completed":  v.progress.CyclesCompleted,
		"items_polled":      v.progress.ItemsPolled,
		"buckets_filled":    v.progress.BucketsFilled,
		"errors":            v.progress.Errors,
		"poll_interval":     v.config.PollInterval.String(),
	}
}
