package collector

import (
	"context"
	"math"
	"sync"
	"time"

	"osrs-flipping/pkg/logging"
)

// SignalComputerConfig configures the signal computation service.
type SignalComputerConfig struct {
	Interval time.Duration // How often to compute signals (default: 5m)
	TTL      time.Duration // How long signals remain active (default: 10m)
}

// DefaultSignalComputerConfig returns sensible defaults.
func DefaultSignalComputerConfig() *SignalComputerConfig {
	return &SignalComputerConfig{
		Interval: 5 * time.Minute,
		TTL:      10 * time.Minute,
	}
}

// SignalComputer periodically computes trading signals and writes them to the DB.
type SignalComputer struct {
	repo   *Repository
	config *SignalComputerConfig
	logger *logging.Logger

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// NewSignalComputer creates a new SignalComputer.
func NewSignalComputer(repo *Repository, config *SignalComputerConfig, logger *logging.Logger) *SignalComputer {
	if config == nil {
		config = DefaultSignalComputerConfig()
	}
	if logger == nil {
		logger = logging.NewLogger("error", "json")
	}
	return &SignalComputer{
		repo:   repo,
		config: config,
		logger: logger,
	}
}

// Start begins the computation loop in a goroutine.
func (sc *SignalComputer) Start() {
	sc.mu.Lock()
	if sc.running {
		sc.mu.Unlock()
		return
	}
	sc.running = true
	sc.stopCh = make(chan struct{})
	sc.doneCh = make(chan struct{})
	sc.mu.Unlock()

	go sc.run()
}

// Stop signals the computer to stop and waits for it to finish.
func (sc *SignalComputer) Stop() {
	sc.mu.Lock()
	if !sc.running {
		sc.mu.Unlock()
		return
	}
	sc.mu.Unlock()

	close(sc.stopCh)
	<-sc.doneCh
}

func (sc *SignalComputer) run() {
	defer func() {
		sc.mu.Lock()
		sc.running = false
		sc.mu.Unlock()
		close(sc.doneCh)
	}()

	sc.logger.WithComponent("signal_computer").WithFields(map[string]interface{}{
		"interval": sc.config.Interval.String(),
		"ttl":      sc.config.TTL.String(),
	}).Info("starting signal computer")

	// Run immediately on start
	sc.compute()

	ticker := time.NewTicker(sc.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-sc.stopCh:
			sc.logger.WithComponent("signal_computer").Info("signal computer stopped")
			return
		case <-ticker.C:
			sc.compute()
		}
	}
}

func (sc *SignalComputer) compute() {
	if sc.repo == nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Allow cancellation via stopCh
	go func() {
		select {
		case <-sc.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	start := time.Now()

	// 1. Clean up expired signals
	deleted, err := sc.repo.DeleteExpiredSignals(ctx)
	if err != nil {
		sc.logger.WithComponent("signal_computer").WithError(err).Error("failed to delete expired signals")
		return
	}
	if deleted > 0 {
		sc.logger.WithComponent("signal_computer").WithField("deleted", deleted).Debug("cleaned expired signals")
	}

	// 2. Compute spread widening signals
	signals, err := sc.computeSpreadWidening(ctx)
	if err != nil {
		sc.logger.WithComponent("signal_computer").WithError(err).Error("failed to compute spread widening signals")
		return
	}

	// 3. Compute price inversion signals
	inversionSignals, err := sc.computePriceInversion(ctx)
	if err != nil {
		sc.logger.WithComponent("signal_computer").WithError(err).Error("failed to compute price inversion signals")
		return
	}
	signals = append(signals, inversionSignals...)

	// 4. Upsert computed signals
	if len(signals) > 0 {
		upserted, err := sc.repo.UpsertSignals(ctx, signals)
		if err != nil {
			sc.logger.WithComponent("signal_computer").WithError(err).Error("failed to upsert signals")
			return
		}

		sc.logger.WithComponent("signal_computer").WithFields(map[string]interface{}{
			"signals_upserted": upserted,
			"duration":         time.Since(start).String(),
		}).Info("signal computation completed")
	} else {
		sc.logger.WithComponent("signal_computer").WithField("duration", time.Since(start).String()).Debug("signal computation completed (no signals)")
	}
}

func (sc *SignalComputer) computeSpreadWidening(ctx context.Context) ([]Signal, error) {
	candidates, err := sc.repo.GetSpreadWideningCandidates(ctx)
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(sc.config.TTL)
	signals := make([]Signal, 0, len(candidates))

	for _, c := range candidates {
		if c.AvgSpread <= 0 {
			continue
		}

		ratio := float64(c.CurrentSpread) / c.AvgSpread
		// Score: linear scale from 1.5x (0.25) to 3x (1.0)
		score := math.Min(1.0, (ratio-1.0)/2.0)
		if score <= 0 {
			continue
		}

		signals = append(signals, Signal{
			ItemID:     c.ItemID,
			SignalType: "spread_widening",
			Score:      score,
			Metadata: map[string]interface{}{
				"current_spread": c.CurrentSpread,
				"avg_spread":     c.AvgSpread,
				"ratio":          math.Round(ratio*100) / 100,
				"high_price":     c.HighPrice,
				"low_price":      c.LowPrice,
				"item_name":      c.ItemName,
				"buy_limit":      c.BuyLimit,
			},
			ExpiresAt: expiresAt,
		})
	}

	return signals, nil
}

func (sc *SignalComputer) computePriceInversion(ctx context.Context) ([]Signal, error) {
	candidates, err := sc.repo.GetInversionCandidates(ctx)
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(sc.config.TTL)
	signals := make([]Signal, 0, len(candidates))

	for _, c := range candidates {
		// Score based on volume spike magnitude: min(1.0, (ratio - 1) / 10)
		score := math.Min(1.0, (c.VolumeRatio-1.0)/10.0)
		if score <= 0 {
			continue
		}

		signals = append(signals, Signal{
			ItemID:     c.ItemID,
			SignalType: "price_inversion",
			Score:      score,
			Metadata: map[string]interface{}{
				"high_price":      c.HighPrice,
				"low_price":       c.LowPrice,
				"spread":          c.Spread,
				"volume":          c.Volume,
				"baseline_volume": math.Round(c.BaselineVolume*100) / 100,
				"volume_ratio":    math.Round(c.VolumeRatio*100) / 100,
				"item_name":       c.ItemName,
				"buy_limit":       c.BuyLimit,
			},
			ExpiresAt: expiresAt,
		})
	}

	return signals, nil
}
