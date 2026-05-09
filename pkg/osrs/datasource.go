package osrs

import (
	"context"
	"fmt"
	"time"

	"osrs-flipping/pkg/storage"
)

// DataSource defines the interface for loading price and volume data.
// Implementations can fetch from API, database, or hybrid sources.
type DataSource interface {
	// LoadPrices returns the latest price data for all items.
	// Also returns item metadata (name, buy_limit, members).
	LoadPrices(ctx context.Context) ([]ItemData, error)

	// LoadVolumeData loads volume metrics for the specified items.
	// Updates the items in place with volume data.
	LoadVolumeData(ctx context.Context, items []ItemData, maxItems int) error

	// IsFresh returns true if the data source has fresh data.
	IsFresh(ctx context.Context) (bool, error)

	// Name returns a description of this data source.
	Name() string
}

// APIDataSource fetches data directly from the OSRS Wiki API.
type APIDataSource struct {
	client *Client
}

// NewAPIDataSource creates a data source that uses the OSRS Wiki API.
func NewAPIDataSource(userAgent string) *APIDataSource {
	return &APIDataSource{
		client: NewClient(userAgent),
	}
}

// NewAPIDataSourceWithClient creates a data source with an existing client.
func NewAPIDataSourceWithClient(client *Client) *APIDataSource {
	return &APIDataSource{
		client: client,
	}
}

func (s *APIDataSource) Name() string {
	return "OSRS Wiki API"
}

func (s *APIDataSource) IsFresh(ctx context.Context) (bool, error) {
	// API is always considered "fresh" - it's the source of truth
	return true, nil
}

func (s *APIDataSource) LoadPrices(ctx context.Context) ([]ItemData, error) {
	// Get item mappings
	mappings, err := s.client.GetItemMapping(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting item mappings: %w", err)
	}

	// Get latest prices
	prices, err := s.client.GetLatestPrices(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("getting latest prices: %w", err)
	}

	// Merge data
	items := mergePricesWithMappings(prices, mappings)

	// Compute derived columns
	computeDerivedColumns(items)

	return items, nil
}

func (s *APIDataSource) LoadVolumeData(ctx context.Context, items []ItemData, maxItems int) error {
	// Extract item IDs
	itemIDs := make([]int, len(items))
	for i, item := range items {
		itemIDs[i] = item.ItemID
	}

	if maxItems > 0 && len(itemIDs) > maxItems {
		itemIDs = itemIDs[:maxItems]
	}

	// Use the volume loading logic from the existing analyzer
	volumeData, err := s.loadVolumeMetricsFromAPI(ctx, itemIDs)
	if err != nil {
		return fmt.Errorf("loading volume metrics: %w", err)
	}

	// Apply metrics to items
	applyVolumeMetrics(items, volumeData)

	return nil
}

// loadVolumeMetricsFromAPI fetches volume data for items from the API.
// This is a simplified version that doesn't use the Analyzer's rate limiter.
func (s *APIDataSource) loadVolumeMetricsFromAPI(ctx context.Context, itemIDs []int) (map[int]VolumeMetrics, error) {
	fmt.Printf("📈 Fetching volume data for %d items from API...\n", len(itemIDs))

	rateLimiter := NewRateLimiter(2.0)
	volumeData := make(map[int]VolumeMetrics)

	for i, itemID := range itemIDs {
		if err := rateLimiter.Wait(ctx); err != nil {
			return volumeData, fmt.Errorf("rate limit wait: %w", err)
		}

		metrics, err := s.calculateVolumeMetrics(ctx, itemID)
		if err != nil {
			fmt.Printf("  ❌ Error fetching data for item %d: %v\n", itemID, err)
			continue
		}

		volumeData[itemID] = metrics

		if (i+1)%50 == 0 || i+1 == len(itemIDs) {
			fmt.Printf("  📊 Progress: %d/%d items processed\n", i+1, len(itemIDs))
		}
	}

	fmt.Printf("✅ Successfully enriched %d/%d items with volume data\n", len(volumeData), len(itemIDs))
	return volumeData, nil
}

// calculateVolumeMetrics processes timeseries data for a single item.
func (s *APIDataSource) calculateVolumeMetrics(ctx context.Context, itemID int) (VolumeMetrics, error) {
	data5m, err := s.client.GetTimeseries(ctx, itemID, "5m")
	if err != nil {
		return VolumeMetrics{}, fmt.Errorf("fetching 5m data: %w", err)
	}

	data24h, err := s.client.GetTimeseries(ctx, itemID, "24h")
	if err != nil {
		return VolumeMetrics{}, fmt.Errorf("fetching 24h data: %w", err)
	}

	return processTimeseriesData(data5m, data24h), nil
}

// DBDataSource fetches data from the local PostgreSQL database.
type DBDataSource struct {
	queryRepo      *storage.QueryRepository
	mappingCache   map[int]ItemMapping
	mappingClient  *Client // For fetching item mappings (not price data)
	freshThreshold time.Duration
}

// NewDBDataSource creates a data source that uses the local database.
// mappingClient is used to fetch item metadata (names, limits) which isn't stored in DB yet.
func NewDBDataSource(queryRepo *storage.QueryRepository, mappingClient *Client, freshThreshold time.Duration) *DBDataSource {
	return &DBDataSource{
		queryRepo:      queryRepo,
		mappingClient:  mappingClient,
		freshThreshold: freshThreshold,
	}
}

func (s *DBDataSource) Name() string {
	return "Local Database"
}

func (s *DBDataSource) IsFresh(ctx context.Context) (bool, error) {
	return s.queryRepo.IsDataFresh(ctx, s.freshThreshold)
}

func (s *DBDataSource) LoadPrices(ctx context.Context) ([]ItemData, error) {
	// Get latest prices from DB
	prices, err := s.queryRepo.GetLatestPrices(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying latest prices: %w", err)
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("no price data in database")
	}

	// Get item mappings (still from API for now - item metadata isn't in DB yet)
	// TODO: Once item_metadata table exists, query from DB
	if s.mappingCache == nil {
		mappings, err := s.mappingClient.GetItemMapping(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting item mappings: %w", err)
		}
		s.mappingCache = make(map[int]ItemMapping)
		for _, m := range mappings {
			s.mappingCache[m.ID] = m
		}
	}

	// Convert to ItemData
	items := make([]ItemData, 0, len(prices))
	for _, p := range prices {
		mapping, ok := s.mappingCache[p.ItemID]
		if !ok {
			continue // Skip items without metadata
		}

		item := ItemData{
			ItemID:            p.ItemID,
			Name:              mapping.Name,
			InstaBuyPrice:     p.HighPrice,
			InstaSellPrice:    p.LowPrice,
			LastInstaBuyTime:  p.HighTime,
			LastInstaSellTime: p.LowTime,
			BuyLimit:          mapping.BuyLimit,
			Members:           mapping.Members,
		}
		items = append(items, item)
	}

	// Compute derived columns
	computeDerivedColumns(items)

	return items, nil
}

func (s *DBDataSource) LoadVolumeData(ctx context.Context, items []ItemData, maxItems int) error {
	// Extract item IDs
	itemIDs := make([]int, len(items))
	for i, item := range items {
		itemIDs[i] = item.ItemID
	}

	if maxItems > 0 && len(itemIDs) > maxItems {
		itemIDs = itemIDs[:maxItems]
	}

	// Get multi-period metrics from DB
	multiMetrics, err := s.queryRepo.GetMultiPeriodVolumeMetrics(ctx, itemIDs)
	if err != nil {
		return fmt.Errorf("querying volume metrics: %w", err)
	}

	// Create item lookup map
	itemMap := make(map[int]*ItemData)
	for i := range items {
		itemMap[items[i].ItemID] = &items[i]
	}

	// Apply metrics to items
	for itemID, mm := range multiMetrics {
		item, ok := itemMap[itemID]
		if !ok {
			continue
		}

		// 20-minute metrics
		if mm.Metrics20m != nil {
			if mm.Metrics20m.HighPriceVolume != nil {
				v := float64(*mm.Metrics20m.HighPriceVolume)
				item.InstaBuyVolume20m = &v
			}
			if mm.Metrics20m.LowPriceVolume != nil {
				v := float64(*mm.Metrics20m.LowPriceVolume)
				item.InstaSellVolume20m = &v
			}
			if mm.Metrics20m.AvgHighPrice != nil {
				v := float64(*mm.Metrics20m.AvgHighPrice)
				item.AvgInstaBuyPrice20m = &v
			}
			if mm.Metrics20m.AvgLowPrice != nil {
				v := float64(*mm.Metrics20m.AvgLowPrice)
				item.AvgInstaSellPrice20m = &v
			}
		}

		// 1-hour metrics
		if mm.Metrics1h != nil {
			if mm.Metrics1h.HighPriceVolume != nil {
				v := float64(*mm.Metrics1h.HighPriceVolume)
				item.InstaBuyVolume1h = &v
			}
			if mm.Metrics1h.LowPriceVolume != nil {
				v := float64(*mm.Metrics1h.LowPriceVolume)
				item.InstaSellVolume1h = &v
			}
			if mm.Metrics1h.AvgHighPrice != nil {
				v := float64(*mm.Metrics1h.AvgHighPrice)
				item.AvgInstaBuyPrice1h = &v
			}
			if mm.Metrics1h.AvgLowPrice != nil {
				v := float64(*mm.Metrics1h.AvgLowPrice)
				item.AvgInstaSellPrice1h = &v
			}
		}

		// 24-hour metrics
		if mm.Metrics24h != nil {
			if mm.Metrics24h.HighPriceVolume != nil {
				v := float64(*mm.Metrics24h.HighPriceVolume)
				item.InstaBuyVolume24h = &v
			}
			if mm.Metrics24h.LowPriceVolume != nil {
				v := float64(*mm.Metrics24h.LowPriceVolume)
				item.InstaSellVolume24h = &v
			}
			if mm.Metrics24h.AvgHighPrice != nil {
				v := float64(*mm.Metrics24h.AvgHighPrice)
				item.AvgInstaBuyPrice24h = &v
			}
			if mm.Metrics24h.AvgLowPrice != nil {
				v := float64(*mm.Metrics24h.AvgLowPrice)
				item.AvgInstaSellPrice24h = &v
			}
		}

		// Compute average margin for periods where we have both prices
		if item.AvgInstaBuyPrice20m != nil && item.AvgInstaSellPrice20m != nil {
			v := *item.AvgInstaBuyPrice20m - *item.AvgInstaSellPrice20m
			item.AvgMarginGP20m = &v
		}
		if item.AvgInstaBuyPrice1h != nil && item.AvgInstaSellPrice1h != nil {
			v := *item.AvgInstaBuyPrice1h - *item.AvgInstaSellPrice1h
			item.AvgMarginGP1h = &v
		}
		if item.AvgInstaBuyPrice24h != nil && item.AvgInstaSellPrice24h != nil {
			v := *item.AvgInstaBuyPrice24h - *item.AvgInstaSellPrice24h
			item.AvgMarginGP24h = &v
		}
	}

	return nil
}

// HybridDataSource uses DB as primary, falling back to API when data is stale.
type HybridDataSource struct {
	dbSource  *DBDataSource
	apiSource *APIDataSource
}

// NewHybridDataSource creates a data source that tries DB first, then falls back to API.
func NewHybridDataSource(dbSource *DBDataSource, apiSource *APIDataSource) *HybridDataSource {
	return &HybridDataSource{
		dbSource:  dbSource,
		apiSource: apiSource,
	}
}

func (s *HybridDataSource) Name() string {
	return "Hybrid (DB + API fallback)"
}

func (s *HybridDataSource) IsFresh(ctx context.Context) (bool, error) {
	// Check if DB is fresh
	dbFresh, err := s.dbSource.IsFresh(ctx)
	if err != nil {
		return false, err
	}
	if dbFresh {
		return true, nil
	}
	// API is always fresh
	return true, nil
}

func (s *HybridDataSource) LoadPrices(ctx context.Context) ([]ItemData, error) {
	// Check if DB data is fresh
	fresh, err := s.dbSource.IsFresh(ctx)
	if err != nil {
		fmt.Printf("Warning: could not check DB freshness: %v, falling back to API\n", err)
		return s.apiSource.LoadPrices(ctx)
	}

	if fresh {
		items, err := s.dbSource.LoadPrices(ctx)
		if err != nil {
			fmt.Printf("Warning: DB load failed: %v, falling back to API\n", err)
			return s.apiSource.LoadPrices(ctx)
		}
		fmt.Printf("✅ Loaded %d items from local database\n", len(items))
		return items, nil
	}

	fmt.Println("Local data is stale, fetching from API...")
	return s.apiSource.LoadPrices(ctx)
}

func (s *HybridDataSource) LoadVolumeData(ctx context.Context, items []ItemData, maxItems int) error {
	// Check if DB data is fresh
	fresh, err := s.dbSource.IsFresh(ctx)
	if err != nil {
		fmt.Printf("Warning: could not check DB freshness: %v, falling back to API\n", err)
		return s.apiSource.LoadVolumeData(ctx, items, maxItems)
	}

	if fresh {
		err := s.dbSource.LoadVolumeData(ctx, items, maxItems)
		if err != nil {
			fmt.Printf("Warning: DB volume load failed: %v, falling back to API\n", err)
			return s.apiSource.LoadVolumeData(ctx, items, maxItems)
		}
		return nil
	}

	return s.apiSource.LoadVolumeData(ctx, items, maxItems)
}

// Helper functions shared between data sources

func mergePricesWithMappings(prices *LatestPricesResponse, mappings []ItemMapping) []ItemData {
	itemMap := make(map[int]ItemMapping)
	for _, m := range mappings {
		itemMap[m.ID] = m
	}

	var items []ItemData
	for itemIDStr, priceInfo := range prices.Data {
		var itemID int
		fmt.Sscanf(itemIDStr, "%d", &itemID)

		mapping, ok := itemMap[itemID]
		if !ok {
			continue
		}

		item := ItemData{
			ItemID:         itemID,
			Name:           mapping.Name,
			InstaBuyPrice:  priceInfo.High,
			InstaSellPrice: priceInfo.Low,
			BuyLimit:       mapping.BuyLimit,
			Members:        mapping.Members,
		}

		if priceInfo.HighTime != nil {
			t := time.Unix(int64(*priceInfo.HighTime), 0).UTC()
			item.LastInstaBuyTime = &t
		}
		if priceInfo.LowTime != nil {
			t := time.Unix(int64(*priceInfo.LowTime), 0).UTC()
			item.LastInstaSellTime = &t
		}

		items = append(items, item)
	}

	return items
}

func computeDerivedColumns(items []ItemData) {
	for i := range items {
		item := &items[i]

		if item.InstaBuyPrice != nil && item.InstaSellPrice != nil {
			marginGP := *item.InstaBuyPrice - *item.InstaSellPrice
			item.MarginGP = marginGP

			if *item.InstaSellPrice > 0 {
				item.MarginPct = (float64(marginGP) / float64(*item.InstaSellPrice)) * 100
			}

			if item.BuyLimit > 0 {
				item.FlipEfficiency = float64(marginGP * item.BuyLimit)
			}
		}
	}
}

func applyVolumeMetrics(items []ItemData, metrics map[int]VolumeMetrics) {
	for i := range items {
		item := &items[i]
		m, ok := metrics[item.ItemID]
		if !ok {
			continue
		}

		// 20-minute metrics
		item.InstaBuyVolume20m = &m.InstaBuyVolume20m
		item.InstaSellVolume20m = &m.InstaSellVolume20m
		item.AvgInstaBuyPrice20m = &m.AvgInstaBuyPrice20m
		item.AvgInstaSellPrice20m = &m.AvgInstaSellPrice20m
		item.AvgMarginGP20m = &m.AvgMarginGP20m

		// 1-hour metrics
		item.InstaBuyVolume1h = &m.InstaBuyVolume1h
		item.InstaSellVolume1h = &m.InstaSellVolume1h
		item.AvgInstaBuyPrice1h = &m.AvgInstaBuyPrice1h
		item.AvgInstaSellPrice1h = &m.AvgInstaSellPrice1h
		item.AvgMarginGP1h = &m.AvgMarginGP1h

		// 24-hour metrics
		item.InstaBuyVolume24h = &m.InstaBuyVolume24h
		item.InstaSellVolume24h = &m.InstaSellVolume24h
		item.AvgInstaBuyPrice24h = &m.AvgInstaBuyPrice24h
		item.AvgInstaSellPrice24h = &m.AvgInstaSellPrice24h
		item.AvgMarginGP24h = &m.AvgMarginGP24h

		// Trend data
		item.InstaSellPriceTrend1h = &m.InstaSellPriceTrend1h
		item.InstaBuyPriceTrend1h = &m.InstaBuyPriceTrend1h
		item.InstaSellPriceTrend24h = &m.InstaSellPriceTrend24h
		item.InstaBuyPriceTrend24h = &m.InstaBuyPriceTrend24h
		item.InstaSellPriceTrend1w = &m.InstaSellPriceTrend1w
		item.InstaBuyPriceTrend1w = &m.InstaBuyPriceTrend1w
		item.InstaSellPriceTrend1m = &m.InstaSellPriceTrend1m
		item.InstaBuyPriceTrend1m = &m.InstaBuyPriceTrend1m
	}
}

// processTimeseriesData converts raw API responses to VolumeMetrics.
// Shared helper extracted from analyzer logic.
func processTimeseriesData(data5m, data24h map[string]interface{}) VolumeMetrics {
	var metrics VolumeMetrics

	// Parse 5-minute data for recent metrics
	if dataSlice, ok := data5m["data"].([]interface{}); ok {
		metrics = process5mData(dataSlice, metrics)
	}

	// Parse 24h data for trend analysis
	if dataSlice, ok := data24h["data"].([]interface{}); ok {
		metrics = process24hData(dataSlice, metrics)
	}

	return metrics
}

func process5mData(dataSlice []interface{}, metrics VolumeMetrics) VolumeMetrics {
	now := time.Now().Unix()
	window20m := now - (20 * 60)
	window1h := now - (60 * 60)
	window24h := now - (24 * 60 * 60)

	var (
		instaBuy20m, instaSell20m       []float64
		instaBuyVol20m, instaSellVol20m float64
		instaBuy1h, instaSell1h         []float64
		instaBuyVol1h, instaSellVol1h   float64
		instaBuy24h, instaSell24h       []float64
		instaBuyVol24h, instaSellVol24h float64
		timestamps1h, instaBuyPrices1h, instaSellPrices1h   []float64
		timestamps24h, instaBuyPrices24h, instaSellPrices24h []float64
	)

	for _, item := range dataSlice {
		if dataPoint, ok := item.(map[string]interface{}); ok {
			timestamp := int64(dataPoint["timestamp"].(float64))

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

	// Calculate trends
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

func process24hData(dataSlice []interface{}, metrics VolumeMetrics) VolumeMetrics {
	now := time.Now().Unix()
	window1w := now - (7 * 24 * 60 * 60)
	window1m := now - (30 * 24 * 60 * 60)

	var (
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

			if timestamp >= window1w {
				if avgHigh > 0 {
					timestamps1w = append(timestamps1w, float64(timestamp))
					instaBuyPrices1w = append(instaBuyPrices1w, avgHigh)
				}
				if avgLow > 0 {
					instaSellPrices1w = append(instaSellPrices1w, avgLow)
				}
			}

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

	// Calculate trends
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
