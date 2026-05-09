package collector

import (
	"context"
	"math"
	"sync"
	"time"

	"osrs-flipping/pkg/logging"
)

// volumeConfidence maps an RVOL (Relative Volume) ratio to a 0–1 confidence score.
//   - 0 or no data → 0 (unknown)
//   - 0.5 → ~0.25 (weak)
//   - 1.0 → 0.5 (baseline)
//   - 2.0 → 0.75 (strong)
//   - 3.0+ → 1.0 (very strong)
func volumeConfidence(ratio float64) float64 {
	if ratio <= 0 {
		return 0
	}
	if ratio >= 3.0 {
		return 1.0
	}
	// Piecewise linear: 0→0, 1→0.5, 3→1.0
	if ratio <= 1.0 {
		return ratio * 0.5
	}
	return 0.5 + (ratio-1.0)*0.25
}

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

	// 4. Fetch price series once for both MACD and RSI (RSI needs 15, MACD needs 35)
	priceSeries, err := sc.repo.GetPriceSeries(ctx, 15)
	if err != nil {
		sc.logger.WithComponent("signal_computer").WithError(err).Error("failed to get price series")
		return
	}

	// 4a. Compute MACD crossover signals
	macdSignals := sc.computeMACD(priceSeries)
	signals = append(signals, macdSignals...)

	// 4b. Compute RSI oversold signals
	rsiSignals := sc.computeRSI(priceSeries)
	signals = append(signals, rsiSignals...)

	// 5a. Manage volume polling for compounded items (non-fatal)
	compoundedIDs, err := sc.repo.GetCompoundedItemIDs(ctx)
	if err != nil {
		sc.logger.WithComponent("signal_computer").WithError(err).Warn("failed to get compounded item IDs for volume polling")
	} else if len(compoundedIDs) > 0 {
		enabled, err := sc.repo.SetPollVolumeAuto(ctx, compoundedIDs)
		if err != nil {
			sc.logger.WithComponent("signal_computer").WithError(err).Warn("failed to auto-enable volume polling")
		} else if enabled > 0 {
			sc.logger.WithComponent("signal_computer").WithField("enabled", enabled).Info("auto-enabled volume polling for compounded items")
		}
	}
	// Disable auto-polled items that are no longer compounded (with grace period)
	if err == nil {
		disabled, err := sc.repo.DisableAutoPolledVolume(ctx, compoundedIDs, 30*time.Minute)
		if err != nil {
			sc.logger.WithComponent("signal_computer").WithError(err).Warn("failed to disable stale auto volume polling")
		} else if disabled > 0 {
			sc.logger.WithComponent("signal_computer").WithField("disabled", disabled).Info("disabled stale auto volume polling")
		}
	}

	// 5b. Annotate volume metadata on compounded items' signals
	if len(compoundedIDs) > 0 {
		volumeRatios, err := sc.repo.GetVolumeRatios(ctx, compoundedIDs)
		if err != nil {
			sc.logger.WithComponent("signal_computer").WithError(err).Warn("failed to get volume ratios")
		} else if len(volumeRatios) > 0 {
			// Build set of compounded items for quick lookup
			compoundedSet := make(map[int]bool, len(compoundedIDs))
			for _, id := range compoundedIDs {
				compoundedSet[id] = true
			}
			for i := range signals {
				if !compoundedSet[signals[i].ItemID] {
					continue
				}
				vr, ok := volumeRatios[signals[i].ItemID]
				if !ok {
					continue
				}
				signals[i].Metadata["volume_ratio"] = math.Round(vr.Ratio*100) / 100
				signals[i].Metadata["volume_confidence"] = math.Round(volumeConfidence(vr.Ratio)*1000) / 1000
			}
		}
	}

	// 6. Upsert computed signals
	if len(signals) > 0 {
		upserted, err := sc.repo.UpsertSignals(ctx, signals)
		if err != nil {
			sc.logger.WithComponent("signal_computer").WithError(err).Error("failed to upsert signals")
			return
		}

		sc.logger.WithComponent("signal_computer").WithFields(map[string]interface{}{
			"signals_upserted": upserted,
			"compounded_items": len(compoundedIDs),
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

// sma computes the simple moving average of the last n values in a price series.
func sma(prices []int, n int) float64 {
	start := len(prices) - n
	if start < 0 {
		start = 0
	}
	window := prices[start:]
	var sum float64
	for _, p := range window {
		sum += float64(p)
	}
	return sum / float64(len(window))
}

// reversionMultiplier computes a score multiplier based on how far the current
// price is below the 24h SMA. Returns 0.5–2.0:
//   - 10%+ below SMA → 2.0x (strong reversion upside)
//   - At SMA → 1.0x (neutral)
//   - 5%+ above SMA → 0.5x (already past mean, dampened)
func reversionMultiplier(currentPrice float64, sma24h float64) float64 {
	if currentPrice <= 0 || sma24h <= 0 {
		return 1.0
	}
	revPct := (sma24h - currentPrice) / currentPrice
	scaled := revPct * 10
	clamped := math.Max(-0.5, math.Min(1.0, scaled))
	return 1 + clamped
}

// ema computes exponential moving average over a price series.
// k = 2 / (period + 1)
func ema(prices []int, period int) []float64 {
	k := 2.0 / float64(period+1)
	result := make([]float64, len(prices))
	result[0] = float64(prices[0])
	for i := 1; i < len(prices); i++ {
		result[i] = float64(prices[i])*k + result[i-1]*(1-k)
	}
	return result
}

// emaFloat computes EMA over float64 values (used for signal line over MACD values).
func emaFloat(values []float64, period int) []float64 {
	k := 2.0 / float64(period+1)
	result := make([]float64, len(values))
	result[0] = values[0]
	for i := 1; i < len(values); i++ {
		result[i] = values[i]*k + result[i-1]*(1-k)
	}
	return result
}

func (sc *SignalComputer) computeMACD(series []ItemPriceSeries) []Signal {
	expiresAt := time.Now().Add(sc.config.TTL)
	var signals []Signal

	for _, item := range series {
		prices := make([]int, len(item.Prices))
		for i, p := range item.Prices {
			prices[i] = p.Price
		}

		// Need at least 26 prices for slow EMA + some data for signal line
		if len(prices) < 35 {
			continue
		}

		fast := ema(prices, 12)
		slow := ema(prices, 26)

		// MACD line = fast EMA - slow EMA
		macdLine := make([]float64, len(prices))
		for i := range prices {
			macdLine[i] = fast[i] - slow[i]
		}

		// Signal line = 9-period EMA of MACD line
		signalLine := emaFloat(macdLine, 9)

		n := len(prices) - 1
		prev := n - 1

		// Bullish crossover: previous MACD < signal, current MACD >= signal
		if macdLine[prev] >= signalLine[prev] || macdLine[n] < signalLine[n] {
			continue
		}

		// Base score = min(1.0, abs(macd - signal) / avg_price * 1000)
		currentPrice := float64(prices[n])
		if currentPrice <= 0 {
			continue
		}
		histogram := macdLine[n] - signalLine[n]
		baseScore := math.Min(1.0, math.Abs(histogram)/currentPrice*1000)
		if baseScore <= 0 {
			continue
		}

		// Apply mean-reversion multiplier: boost when price is below 24h SMA
		sma24h := sma(prices, 24)
		score := math.Min(1.0, baseScore*reversionMultiplier(currentPrice, sma24h))
		revPct := (sma24h - currentPrice) / currentPrice

		signals = append(signals, Signal{
			ItemID:     item.ItemID,
			SignalType: "macd_crossover",
			Score:      math.Round(score*1000) / 1000,
			Metadata: map[string]interface{}{
				"macd":          math.Round(macdLine[n]*100) / 100,
				"signal_line":   math.Round(signalLine[n]*100) / 100,
				"histogram":     math.Round(histogram*100) / 100,
				"fast_ema":      math.Round(fast[n]*100) / 100,
				"slow_ema":      math.Round(slow[n]*100) / 100,
				"sma_24h":       int(math.Round(sma24h)),
				"reversion_pct": math.Round(revPct*10000) / 10000,
				"high_price":    prices[n],
				"low_price":     prices[n],
				"item_name":     item.ItemName,
				"buy_limit":     item.BuyLimit,
			},
			ExpiresAt: expiresAt,
		})
	}

	return signals
}

func (sc *SignalComputer) computeRSI(series []ItemPriceSeries) []Signal {
	expiresAt := time.Now().Add(sc.config.TTL)
	var signals []Signal

	for _, item := range series {
		prices := make([]int, len(item.Prices))
		for i, p := range item.Prices {
			prices[i] = p.Price
		}

		if len(prices) < 15 {
			continue
		}

		// Compute gains and losses
		n := len(prices)
		gains := make([]float64, n-1)
		losses := make([]float64, n-1)
		for i := 1; i < n; i++ {
			diff := float64(prices[i] - prices[i-1])
			if diff > 0 {
				gains[i-1] = diff
			} else {
				losses[i-1] = -diff
			}
		}

		// First average: simple average of first 14 periods
		period := 14
		if len(gains) < period {
			continue
		}

		var avgGain, avgLoss float64
		for i := 0; i < period; i++ {
			avgGain += gains[i]
			avgLoss += losses[i]
		}
		avgGain /= float64(period)
		avgLoss /= float64(period)

		// Wilder's smoothing for remaining periods
		for i := period; i < len(gains); i++ {
			avgGain = (avgGain*float64(period-1) + gains[i]) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + losses[i]) / float64(period)
		}

		// RSI = 100 - (100 / (1 + RS))
		var rsi float64
		if avgLoss == 0 {
			rsi = 100
		} else {
			rs := avgGain / avgLoss
			rsi = 100 - (100 / (1 + rs))
		}

		// Only emit signal when RSI < 30
		if rsi >= 30 {
			continue
		}

		// Base score = (30 - RSI) / 30
		baseScore := (30 - rsi) / 30

		lastPrice := prices[n-1]

		// Apply mean-reversion multiplier: boost when price is below 24h SMA
		sma24h := sma(prices, 24)
		currentPrice := float64(lastPrice)
		score := math.Min(1.0, baseScore*reversionMultiplier(currentPrice, sma24h))
		revPct := 0.0
		if currentPrice > 0 {
			revPct = (sma24h - currentPrice) / currentPrice
		}

		signals = append(signals, Signal{
			ItemID:     item.ItemID,
			SignalType: "rsi_oversold",
			Score:      math.Round(score*1000) / 1000,
			Metadata: map[string]interface{}{
				"rsi":           math.Round(rsi*100) / 100,
				"periods":       period,
				"avg_gain":      math.Round(avgGain*100) / 100,
				"avg_loss":      math.Round(avgLoss*100) / 100,
				"sma_24h":       int(math.Round(sma24h)),
				"reversion_pct": math.Round(revPct*10000) / 10000,
				"high_price":    lastPrice,
				"low_price":     lastPrice,
				"item_name":     item.ItemName,
				"buy_limit":     item.BuyLimit,
			},
			ExpiresAt: expiresAt,
		})
	}

	return signals
}
