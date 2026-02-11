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

	// 4. Compute MACD crossover signals
	macdSignals, err := sc.computeMACD(ctx)
	if err != nil {
		sc.logger.WithComponent("signal_computer").WithError(err).Error("failed to compute MACD signals")
		return
	}
	signals = append(signals, macdSignals...)

	// 5. Compute RSI oversold signals
	rsiSignals, err := sc.computeRSI(ctx)
	if err != nil {
		sc.logger.WithComponent("signal_computer").WithError(err).Error("failed to compute RSI signals")
		return
	}
	signals = append(signals, rsiSignals...)

	// 6. Upsert computed signals
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

func (sc *SignalComputer) computeMACD(ctx context.Context) ([]Signal, error) {
	series, err := sc.repo.GetMACDCandidates(ctx)
	if err != nil {
		return nil, err
	}

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

		// Score = min(1.0, abs(macd - signal) / avg_price * 1000)
		avgPrice := float64(prices[n])
		if avgPrice <= 0 {
			continue
		}
		histogram := macdLine[n] - signalLine[n]
		score := math.Min(1.0, math.Abs(histogram)/avgPrice*1000)
		if score <= 0 {
			continue
		}

		signals = append(signals, Signal{
			ItemID:     item.ItemID,
			SignalType: "macd_crossover",
			Score:      math.Round(score*1000) / 1000,
			Metadata: map[string]interface{}{
				"macd":        math.Round(macdLine[n]*100) / 100,
				"signal_line": math.Round(signalLine[n]*100) / 100,
				"histogram":   math.Round(histogram*100) / 100,
				"fast_ema":    math.Round(fast[n]*100) / 100,
				"slow_ema":    math.Round(slow[n]*100) / 100,
				"high_price":  prices[n],
				"low_price":   prices[n],
				"item_name":   item.ItemName,
				"buy_limit":   item.BuyLimit,
			},
			ExpiresAt: expiresAt,
		})
	}

	return signals, nil
}

func (sc *SignalComputer) computeRSI(ctx context.Context) ([]Signal, error) {
	series, err := sc.repo.GetRSICandidates(ctx)
	if err != nil {
		return nil, err
	}

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

		// Score = (30 - RSI) / 30
		score := (30 - rsi) / 30

		lastPrice := prices[n-1]

		signals = append(signals, Signal{
			ItemID:     item.ItemID,
			SignalType: "rsi_oversold",
			Score:      math.Round(score*1000) / 1000,
			Metadata: map[string]interface{}{
				"rsi":        math.Round(rsi*100) / 100,
				"periods":    period,
				"avg_gain":   math.Round(avgGain*100) / 100,
				"avg_loss":   math.Round(avgLoss*100) / 100,
				"high_price": lastPrice,
				"low_price":  lastPrice,
				"item_name":  item.ItemName,
				"buy_limit":  item.BuyLimit,
			},
			ExpiresAt: expiresAt,
		})
	}

	return signals, nil
}
