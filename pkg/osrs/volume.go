package osrs

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter for API calls
type RateLimiter struct {
	tokens     int
	maxTokens  int
	refillRate time.Duration
	lastRefill time.Time
	mutex      sync.Mutex
}

// NewRateLimiter creates a rate limiter with specified requests per second
func NewRateLimiter(requestsPerSecond float64) *RateLimiter {
	maxTokens := int(math.Ceil(requestsPerSecond))
	refillRate := time.Duration(float64(time.Second) / requestsPerSecond)

	return &RateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available, respecting the rate limit
func (rl *RateLimiter) Wait(ctx context.Context) error {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	// Refill tokens based on time passed
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	tokensToAdd := int(elapsed / rl.refillRate)

	if tokensToAdd > 0 {
		rl.tokens = min(rl.maxTokens, rl.tokens+tokensToAdd)
		rl.lastRefill = now
	}

	// If we have tokens, consume one
	if rl.tokens > 0 {
		rl.tokens--
		return nil
	}

	// Wait until next token is available
	waitTime := rl.refillRate - (elapsed % rl.refillRate)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(waitTime):
		rl.tokens = rl.maxTokens - 1 // Consume the token we just got
		rl.lastRefill = time.Now()
		return nil
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// VolumeDataPoint represents a single timeseries data point
type VolumeDataPoint struct {
	Timestamp    int64 `json:"timestamp"`
	AvgHighPrice *int  `json:"avgHighPrice"`
	AvgLowPrice  *int  `json:"avgLowPrice"`
	HighPriceVol *int  `json:"highPriceVolume"`
	LowPriceVol  *int  `json:"lowPriceVolume"`
}

// TimeseriesResponse represents the API response for timeseries data
type TimeseriesResponse struct {
	Data []VolumeDataPoint `json:"data"`
}

// LoadVolumeData fetches volume data for specified items with rate limiting
// Equivalent to load_volume_data method in Python
func (a *Analyzer) LoadVolumeData(ctx context.Context, itemIDs []int, maxItems int) error {
	if !a.HasData() {
		return fmt.Errorf("no data available. Please load data first with LoadData()")
	}

	// Use top profitable items if none specified
	if itemIDs == nil {
		itemIDs = a.getTopItemIDs(maxItems)
	}

	if len(itemIDs) > maxItems {
		itemIDs = itemIDs[:maxItems]
	}

	fmt.Printf("📈 Fetching volume data for %d items (rate limited to 2 req/sec)...\n", len(itemIDs))

	// Create rate limiter: 2 requests per second with some buffer
	rateLimiter := NewRateLimiter(2.0) // Slightly under 2 req/sec for safety

	// Create worker pool with limited concurrency
	const numWorkers = 2 // Keep concurrency low due to rate limit

	type volumeJob struct {
		itemID int
		index  int
	}

	type volumeResult struct {
		itemID  int
		index   int
		metrics VolumeMetrics
		err     error
	}

	jobs := make(chan volumeJob, len(itemIDs))
	results := make(chan volumeResult, len(itemIDs))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobs {
				// Wait for rate limiter
				if err := rateLimiter.Wait(ctx); err != nil {
					results <- volumeResult{itemID: job.itemID, index: job.index, err: err}
					continue
				}

				// Add small jitter to avoid thundering herd (like your Python version)
				jitterMs := 100 + rand.Intn(200) // 100-300ms jitter
				time.Sleep(time.Duration(jitterMs) * time.Millisecond)

				metrics, err := a.calculateVolumeMetrics(ctx, job.itemID)
				results <- volumeResult{
					itemID:  job.itemID,
					index:   job.index,
					metrics: metrics,
					err:     err,
				}
			}
		}(i)
	}

	// Send jobs
	go func() {
		defer close(jobs)
		for i, itemID := range itemIDs {
			select {
			case jobs <- volumeJob{itemID: itemID, index: i}:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for workers to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results and update items
	successCount := 0
	volumeData := make(map[int]VolumeMetrics)

	for result := range results {
		if result.err != nil {
			fmt.Printf("  ❌ Error fetching data for item %d: %v\n", result.itemID, result.err)
			continue
		}

		volumeData[result.itemID] = result.metrics
		successCount++

		// Progress indicator - every 50 items
		if successCount%50 == 0 || successCount == len(itemIDs) {
			fmt.Printf("  📊 Progress: %d/%d items processed\n", successCount, len(itemIDs))
		}
	}

	// Update analyzer data with volume metrics
	a.updateItemsWithVolumeData(volumeData)

	fmt.Printf("✅ Successfully enriched %d/%d items with volume data\n", successCount, len(itemIDs))
	return nil
}

// calculateVolumeMetrics processes timeseries data for a single item
func (a *Analyzer) calculateVolumeMetrics(ctx context.Context, itemID int) (VolumeMetrics, error) {
	// Get 5-minute data for recent metrics (20m, 1h calculations)
	data5m, err := a.client.GetTimeseries(ctx, itemID, "5m")
	if err != nil {
		return VolumeMetrics{}, fmt.Errorf("fetching 5m data: %w", err)
	}

	// Get 24h data for longer trend analysis
	data24h, err := a.client.GetTimeseries(ctx, itemID, "24h")
	if err != nil {
		return VolumeMetrics{}, fmt.Errorf("fetching 24h data: %w", err)
	}

	// Process the timeseries data
	metrics := a.processTimeseriesData(data5m, data24h)
	return metrics, nil
}

// processTimeseriesData converts raw API response to our metrics
func (a *Analyzer) processTimeseriesData(data5m, data24h map[string]interface{}) VolumeMetrics {
	var metrics VolumeMetrics

	// Parse 5-minute data for recent metrics
	if dataSlice, ok := data5m["data"].([]interface{}); ok {
		metrics = a.calculate5mMetrics(dataSlice, metrics)
	}

	// Parse 24h data for trend analysis
	if dataSlice, ok := data24h["data"].([]interface{}); ok {
		metrics = a.calculate24hMetrics(dataSlice, metrics)
	}

	return metrics
}

// calculate5mMetrics processes 5-minute data for 20m, 1h, and 24h windows
func (a *Analyzer) calculate5mMetrics(dataSlice []interface{}, metrics VolumeMetrics) VolumeMetrics {
	now := time.Now().Unix()

	// Time windows
	window20m := now - (20 * 60)      // 20 minutes ago
	window1h := now - (60 * 60)       // 1 hour ago
	window24h := now - (24 * 60 * 60) // 25 hour ago

	var (
		// 20-minute aggregates
		instaBuy20m, instaSell20m       []float64
		instaBuyVol20m, instaSellVol20m float64

		// 1-hour aggregates
		instaBuy1h, instaSell1h       []float64
		instaBuyVol1h, instaSellVol1h float64

		// 24-hour aggregates
		instaBuy24h, instaSell24h       []float64
		instaBuyVol24h, instaSellVol24h float64

		// For 1h trend analysis - collect timestamps and prices
		timestamps1h, instaBuyPrices1h, instaSellPrices1h []float64

		// For 24h trend analysis - collect timestamps and prices
		timestamps24h, instaBuyPrices24h, instaSellPrices24h []float64
	)

	for _, item := range dataSlice {
		if dataPoint, ok := item.(map[string]interface{}); ok {
			timestamp := int64(dataPoint["timestamp"].(float64))

			// Extract prices and volumes
			var avgHigh, avgLow, highVol, lowVol float64
			if val, exists := dataPoint["avgHighPrice"]; exists && val != nil {
				avgHigh = val.(float64)
			}
			if val, exists := dataPoint["avgLowPrice"]; exists && val != nil {
				avgLow = val.(float64)
			}
			if val, exists := dataPoint["highPriceVolume"]; exists && val != nil {
				highVol = val.(float64)
			}
			if val, exists := dataPoint["lowPriceVolume"]; exists && val != nil {
				lowVol = val.(float64)
			}

			// 20-minute window
			if timestamp >= window20m {
				if avgHigh > 0 {
					instaBuy20m = append(instaBuy20m, avgHigh)
				}
				if avgLow > 0 {
					instaSell20m = append(instaSell20m, avgLow)
				}
				instaBuyVol20m += highVol
				instaSellVol20m += lowVol
			}

			// 1-hour window
			if timestamp >= window1h {
				if avgHigh > 0 {
					instaBuy1h = append(instaBuy1h, avgHigh)
					timestamps1h = append(timestamps1h, float64(timestamp))
					instaBuyPrices1h = append(instaBuyPrices1h, avgHigh)
				}
				if avgLow > 0 {
					instaSell1h = append(instaSell1h, avgLow)
					instaSellPrices1h = append(instaSellPrices1h, avgLow)
				}
				instaBuyVol1h += highVol
				instaSellVol1h += lowVol
			}

			if timestamp >= window24h {
				if avgHigh > 0 {
					instaBuy24h = append(instaBuy24h, avgHigh)
					timestamps24h = append(timestamps24h, float64(timestamp))
					instaBuyPrices24h = append(instaBuyPrices24h, avgHigh)
				}
				if avgLow > 0 {
					instaSell24h = append(instaSell24h, avgLow)
					instaSellPrices24h = append(instaSellPrices24h, avgLow)
				}
				instaBuyVol24h += highVol
				instaSellVol24h += lowVol
			}
		}
	}

	// Calculate averages
	if len(instaBuy20m) > 0 {
		metrics.AvgInstaBuyPrice20m = average(instaBuy20m)
	}
	if len(instaSell20m) > 0 {
		metrics.AvgInstaSellPrice20m = average(instaSell20m)
	}
	metrics.InstaBuyVolume20m = instaBuyVol20m
	metrics.InstaSellVolume20m = instaSellVol20m
	metrics.AvgMarginGP20m = metrics.AvgInstaBuyPrice20m - metrics.AvgInstaSellPrice20m

	if len(instaBuy1h) > 0 {
		metrics.AvgInstaBuyPrice1h = average(instaBuy1h)
	}
	if len(instaSell1h) > 0 {
		metrics.AvgInstaSellPrice1h = average(instaSell1h)
	}
	metrics.InstaBuyVolume1h = instaBuyVol1h
	metrics.InstaSellVolume1h = instaSellVol1h
	metrics.AvgMarginGP1h = metrics.AvgInstaBuyPrice1h - metrics.AvgInstaSellPrice1h

	if len(instaBuy24h) > 0 {
		metrics.AvgInstaBuyPrice24h = average(instaBuy24h)
	}
	if len(instaSell24h) > 0 {
		metrics.AvgInstaSellPrice24h = average(instaSell24h)
	}
	metrics.InstaBuyVolume24h = instaBuyVol24h
	metrics.InstaSellVolume24h = instaSellVol24h
	metrics.AvgMarginGP24h = metrics.AvgInstaBuyPrice24h - metrics.AvgInstaSellPrice24h

	// Calculate 1h trends using linear regression
	if len(instaBuyPrices1h) >= 3 {
		metrics.InstaBuyPriceTrend1h = calculateTrend(timestamps1h, instaBuyPrices1h)
	} else {
		metrics.InstaBuyPriceTrend1h = "flat"
	}

	if len(instaSellPrices1h) >= 3 {
		metrics.InstaSellPriceTrend1h = calculateTrend(timestamps1h, instaSellPrices1h)
	} else {
		metrics.InstaSellPriceTrend1h = "flat"
	}

	// Calculate 24h trends using linear regression
	if len(instaBuyPrices24h) >= 3 {
		metrics.InstaBuyPriceTrend24h = calculateTrend(timestamps24h, instaBuyPrices24h)
	} else {
		metrics.InstaBuyPriceTrend24h = "flat"
	}

	if len(instaSellPrices24h) >= 3 {
		metrics.InstaSellPriceTrend24h = calculateTrend(timestamps24h, instaSellPrices24h)
	} else {
		metrics.InstaSellPriceTrend24h = "flat"
	}

	return metrics
}

// calculate24hMetrics processes 24h data for long-term week and month trend analysis
func (a *Analyzer) calculate24hMetrics(dataSlice []interface{}, metrics VolumeMetrics) VolumeMetrics {
	now := time.Now().Unix()
	window1w := now - (7 * 24 * 60 * 60)  // 1 week ago
	window1m := now - (30 * 24 * 60 * 60) // 1 month ago

	var (
		// For trend analysis - separate arrays for different time periods
		timestamps1w, instaBuyPrices1w, instaSellPrices1w []float64
		timestamps1m, instaBuyPrices1m, instaSellPrices1m []float64
	)

	for _, item := range dataSlice {
		if dataPoint, ok := item.(map[string]interface{}); ok {
			timestamp := int64(dataPoint["timestamp"].(float64))

			var avgHigh, avgLow float64
			if val, exists := dataPoint["avgHighPrice"]; exists && val != nil {
				avgHigh = val.(float64)
			}
			if val, exists := dataPoint["avgLowPrice"]; exists && val != nil {
				avgLow = val.(float64)
			}

			// 1-week window for weekly trend analysis
			if timestamp >= window1w {
				if avgHigh > 0 {
					timestamps1w = append(timestamps1w, float64(timestamp))
					instaBuyPrices1w = append(instaBuyPrices1w, avgHigh)
				}
				if avgLow > 0 {
					instaSellPrices1w = append(instaSellPrices1w, avgLow)
				}
			}

			// 1-month window for monthly trend analysis
			if timestamp >= window1m {
				if avgHigh > 0 {
					timestamps1m = append(timestamps1m, float64(timestamp))
					instaBuyPrices1m = append(instaBuyPrices1m, avgHigh)
				}
				if avgLow > 0 {
					instaSellPrices1m = append(instaSellPrices1m, avgLow)
				}
			}
		}
	}

	// Calculate 1w trends using linear regression
	if len(instaBuyPrices1w) >= 3 {
		metrics.InstaBuyPriceTrend1w = calculateTrend(timestamps1w, instaBuyPrices1w)
	} else {
		metrics.InstaBuyPriceTrend1w = "flat"
	}

	if len(instaSellPrices1w) >= 3 {
		metrics.InstaSellPriceTrend1w = calculateTrend(timestamps1w, instaSellPrices1w)
	} else {
		metrics.InstaSellPriceTrend1w = "flat"
	}

	// Calculate 1m trends using linear regression
	if len(instaBuyPrices1m) >= 3 {
		metrics.InstaBuyPriceTrend1m = calculateTrend(timestamps1m, instaBuyPrices1m)
	} else {
		metrics.InstaBuyPriceTrend1m = "flat"
	}

	if len(instaSellPrices1m) >= 3 {
		metrics.InstaSellPriceTrend1m = calculateTrend(timestamps1m, instaSellPrices1m)
	} else {
		metrics.InstaSellPriceTrend1m = "flat"
	}

	return metrics
}

// getTopItemIDs returns item IDs sorted by flip efficiency for volume analysis
func (a *Analyzer) getTopItemIDs(maxItems int) []int {
	// Filter items with meaningful data
	var candidates []ItemData
	for _, item := range a.items {
		if item.InstaBuyPrice != nil && item.InstaSellPrice != nil &&
			item.MarginGP > 100 && item.BuyLimit > 0 {
			candidates = append(candidates, item)
		}
	}

	// Sort by flip efficiency
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[i].FlipEfficiency < candidates[j].FlipEfficiency {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// Extract top item IDs
	limit := min(maxItems, len(candidates))
	itemIDs := make([]int, limit)
	for i := 0; i < limit; i++ {
		itemIDs[i] = candidates[i].ItemID
	}

	return itemIDs
}

// updateItemsWithVolumeData merges volume metrics back into the main items slice
func (a *Analyzer) updateItemsWithVolumeData(volumeData map[int]VolumeMetrics) {
	for i := range a.items {
		if metrics, exists := volumeData[a.items[i].ItemID]; exists {
			item := &a.items[i]

			// Update 20m metrics
			item.InstaBuyVolume20m = &metrics.InstaBuyVolume20m
			item.InstaSellVolume20m = &metrics.InstaSellVolume20m
			item.AvgInstaBuyPrice20m = &metrics.AvgInstaBuyPrice20m
			item.AvgInstaSellPrice20m = &metrics.AvgInstaSellPrice20m
			item.AvgMarginGP20m = &metrics.AvgMarginGP20m

			// Update 1h metrics
			item.InstaBuyVolume1h = &metrics.InstaBuyVolume1h
			item.InstaSellVolume1h = &metrics.InstaSellVolume1h
			item.AvgInstaBuyPrice1h = &metrics.AvgInstaBuyPrice1h
			item.AvgInstaSellPrice1h = &metrics.AvgInstaSellPrice1h
			item.AvgMarginGP1h = &metrics.AvgMarginGP1h

			// Update 24h metrics
			item.InstaBuyVolume24h = &metrics.InstaBuyVolume24h
			item.InstaSellVolume24h = &metrics.InstaSellVolume24h
			item.AvgInstaBuyPrice24h = &metrics.AvgInstaBuyPrice24h
			item.AvgInstaSellPrice24h = &metrics.AvgInstaSellPrice24h
			item.AvgMarginGP24h = &metrics.AvgMarginGP24h

			// Update trends (all time periods)
			if metrics.InstaBuyPriceTrend1h != "" {
				item.InstaBuyPriceTrend1h = &metrics.InstaBuyPriceTrend1h
			}
			if metrics.InstaSellPriceTrend1h != "" {
				item.InstaSellPriceTrend1h = &metrics.InstaSellPriceTrend1h
			}
			if metrics.InstaBuyPriceTrend24h != "" {
				item.InstaBuyPriceTrend24h = &metrics.InstaBuyPriceTrend24h
			}
			if metrics.InstaSellPriceTrend24h != "" {
				item.InstaSellPriceTrend24h = &metrics.InstaSellPriceTrend24h
			}
			if metrics.InstaBuyPriceTrend1w != "" {
				item.InstaBuyPriceTrend1w = &metrics.InstaBuyPriceTrend1w
			}
			if metrics.InstaSellPriceTrend1w != "" {
				item.InstaSellPriceTrend1w = &metrics.InstaSellPriceTrend1w
			}
			if metrics.InstaBuyPriceTrend1m != "" {
				item.InstaBuyPriceTrend1m = &metrics.InstaBuyPriceTrend1m
			}
			if metrics.InstaSellPriceTrend1m != "" {
				item.InstaSellPriceTrend1m = &metrics.InstaSellPriceTrend1m
			}
		}
	}
}

// Helper functions
func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// calculateTrend performs linear regression to determine price trend
// This matches the Python implementation logic
func calculateTrend(x, y []float64) string {
	// Need at least 3 points for a meaningful trend (matching Python)
	if len(x) < 3 || len(x) != len(y) {
		return "flat"
	}

	// Check for empty values
	if len(y) == 0 {
		return "flat"
	}

	n := float64(len(x))
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0

	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
	}

	// Calculate slope using linear regression (m = (n*ΣXY - ΣX*ΣY) / (n*ΣX² - (ΣX)²))
	numerator := n*sumXY - sumX*sumY
	denominator := n*sumX2 - sumX*sumX

	if denominator == 0 {
		return "flat"
	}

	slope := numerator / denominator

	// Calculate percentage change over the period (matching Python logic)
	var pctChange float64
	if len(y) > 1 && y[0] != 0 {
		pctChange = (y[len(y)-1] - y[0]) / y[0] * 100
	}

	// Determine trend based on slope and percent change
	// Less than 1% change is considered flat (matching Python threshold)
	if math.Abs(pctChange) < 1.0 {
		return "flat"
	} else if math.Abs(pctChange) >= 10.0 {
		// Sharp moves: 10% or more
		if slope > 0 {
			return "sharp increase"
		} else {
			return "sharp decrease"
		}
	} else if slope > 0 {
		return "increasing"
	} else {
		return "decreasing"
	}
}
