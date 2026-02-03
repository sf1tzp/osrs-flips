package collector

import (
	"context"
	"strconv"
	"sync"
	"time"

	"osrs-flipping/pkg/logging"
	"osrs-flipping/pkg/osrs"
)

// PollerConfig configures the polling service.
type PollerConfig struct {
	Interval    time.Duration // Polling interval (default: 60s)
	RetryDelay  time.Duration // Delay between retries on failure (default: 10s)
	MaxRetries  int           // Max consecutive failures before backing off (default: 5)
	BackoffMax  time.Duration // Maximum backoff duration (default: 5m)
}

// DefaultPollerConfig returns sensible defaults.
func DefaultPollerConfig() *PollerConfig {
	return &PollerConfig{
		Interval:   60 * time.Second,
		RetryDelay: 10 * time.Second,
		MaxRetries: 5,
		BackoffMax: 5 * time.Minute,
	}
}

// Poller continuously polls the /latest endpoint and stores observations.
type Poller struct {
	client *osrs.Client
	repo   *Repository
	config *PollerConfig
	logger *logging.Logger

	mu              sync.Mutex
	running         bool
	stopCh          chan struct{}
	consecutiveFails int
}

// NewPoller creates a new Poller.
func NewPoller(client *osrs.Client, repo *Repository, config *PollerConfig, logger *logging.Logger) *Poller {
	if config == nil {
		config = DefaultPollerConfig()
	}
	return &Poller{
		client: client,
		repo:   repo,
		config: config,
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

// Start begins the polling loop in a goroutine.
func (p *Poller) Start() {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.mu.Unlock()

	go p.run()
}

// Stop signals the poller to stop and waits for it to finish.
func (p *Poller) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	close(p.stopCh)
}

func (p *Poller) run() {
	p.logger.WithComponent("poller").Info("starting polling loop")

	// Run immediately on start
	p.poll()

	ticker := time.NewTicker(p.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			p.logger.WithComponent("poller").Info("polling loop stopped")
			p.mu.Lock()
			p.running = false
			p.mu.Unlock()
			return
		case <-ticker.C:
			p.poll()
		}
	}
}

func (p *Poller) poll() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	observedAt := time.Now().UTC()

	p.logger.WithComponent("poller").Debug("fetching latest prices")

	// Fetch latest prices from API
	resp, err := p.client.GetLatestPrices(ctx, nil)
	if err != nil {
		p.handleError(err)
		return
	}

	// Convert API response to observations
	observations := make([]PriceObservation, 0, len(resp.Data))
	for itemIDStr, priceInfo := range resp.Data {
		itemID, err := strconv.Atoi(itemIDStr)
		if err != nil {
			p.logger.WithComponent("poller").WithError(err).WithField("item_id", itemIDStr).Warn("invalid item ID, skipping")
			continue
		}

		obs := PriceObservation{
			ItemID:     itemID,
			ObservedAt: observedAt,
			HighPrice:  priceInfo.High,
			LowPrice:   priceInfo.Low,
		}

		// Convert Unix timestamps to time.Time
		if priceInfo.HighTime != nil {
			t := time.Unix(int64(*priceInfo.HighTime), 0).UTC()
			obs.HighTime = &t
		}
		if priceInfo.LowTime != nil {
			t := time.Unix(int64(*priceInfo.LowTime), 0).UTC()
			obs.LowTime = &t
		}

		observations = append(observations, obs)
	}

	// Insert into database
	inserted, err := p.repo.InsertPriceObservations(ctx, observations)
	if err != nil {
		p.handleError(err)
		return
	}

	// Success - reset failure counter
	p.consecutiveFails = 0

	p.logger.WithComponent("poller").WithFields(map[string]interface{}{
		"items_fetched": len(resp.Data),
		"rows_inserted": inserted,
		"observed_at":   observedAt.Format(time.RFC3339),
	}).Info("poll completed")
}

func (p *Poller) handleError(err error) {
	p.consecutiveFails++

	p.logger.WithComponent("poller").WithError(err).WithField("consecutive_fails", p.consecutiveFails).Error("poll failed")

	// Implement exponential backoff if too many failures
	if p.consecutiveFails >= p.config.MaxRetries {
		backoff := time.Duration(p.consecutiveFails-p.config.MaxRetries+1) * p.config.RetryDelay
		if backoff > p.config.BackoffMax {
			backoff = p.config.BackoffMax
		}
		p.logger.WithComponent("poller").WithField("backoff", backoff).Warn("backing off due to repeated failures")
		time.Sleep(backoff)
	}
}

// Stats returns current poller statistics.
func (p *Poller) Stats() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return map[string]interface{}{
		"running":           p.running,
		"consecutive_fails": p.consecutiveFails,
		"interval":          p.config.Interval.String(),
	}
}
