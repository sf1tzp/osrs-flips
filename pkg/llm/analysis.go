package llm

import (
	"encoding/json"
	"fmt"
	"time"

	"osrs-flipping/pkg/osrs"
)

// TradingAnalysisRequest represents a structured request for LLM trading analysis
type TradingAnalysisRequest struct {
	Timestamp   time.Time        `json:"timestamp"`
	Summary     TradingSummary   `json:"summary"`
	TopItems    []osrs.ItemData  `json:"top_items"`
	MarketStats MarketStatistics `json:"market_stats"`
}

// TradingSummary provides high-level market insights
type TradingSummary struct {
	TotalItemsAnalyzed int     `json:"total_items_analyzed"`
	HighestMarginGP    int     `json:"highest_margin_gp"`
	AverageMarginGP    float64 `json:"average_margin_gp"`
	TotalOpportunities int     `json:"total_opportunities"`
}

// MarketStatistics provides statistical insights
type MarketStatistics struct {
	ItemsAbove1MGP    int     `json:"items_above_1m_gp"`
	ItemsAbove100KGP  int     `json:"items_above_100k_gp"`
	HighVolumeItems   int     `json:"high_volume_items"`
	RecentlyUpdated   int     `json:"recently_updated"`
	AvgFlipEfficiency float64 `json:"avg_flip_efficiency"`
}

// CreateTradingAnalysisRequest builds a structured analysis request from filtered items
func CreateTradingAnalysisRequest(items []osrs.ItemData, totalItems int) TradingAnalysisRequest {
	if len(items) == 0 {
		return TradingAnalysisRequest{
			Timestamp: time.Now(),
			Summary: TradingSummary{
				TotalItemsAnalyzed: totalItems,
			},
			TopItems: []osrs.ItemData{},
		}
	}

	// Calculate statistics
	var totalMargin int64
	var totalEfficiency float64
	itemsAbove1M := 0
	itemsAbove100K := 0
	highVolume := 0
	recentlyUpdated := 0

	for _, item := range items {
		totalMargin += int64(item.MarginGP)
		totalEfficiency += item.FlipEfficiency

		if item.MarginGP >= 1000000 {
			itemsAbove1M++
		}
		if item.MarginGP >= 100000 {
			itemsAbove100K++
		}

		// Check for high volume (if volume data available)
		if item.InstaBuyVolume1h != nil && item.InstaSellVolume1h != nil {
			if *item.InstaBuyVolume1h > 100 && *item.InstaSellVolume1h > 100 {
				highVolume++
			}
		}

		// Check if recently updated (within last hour)
		now := time.Now()
		if item.LastInstaBuyTime != nil && now.Sub(*item.LastInstaBuyTime) < time.Hour {
			recentlyUpdated++
		}
		if item.LastInstaSellTime != nil && now.Sub(*item.LastInstaSellTime) < time.Hour {
			recentlyUpdated++
		}
	}

	avgMargin := float64(totalMargin) / float64(len(items))
	avgEfficiency := totalEfficiency / float64(len(items))

	return TradingAnalysisRequest{
		Timestamp: time.Now(),
		Summary: TradingSummary{
			TotalItemsAnalyzed: totalItems,
			HighestMarginGP:    items[0].MarginGP, // Assuming sorted by margin
			AverageMarginGP:    avgMargin,
			TotalOpportunities: len(items),
		},
		TopItems: items,
		MarketStats: MarketStatistics{
			ItemsAbove1MGP:    itemsAbove1M,
			ItemsAbove100KGP:  itemsAbove100K,
			HighVolumeItems:   highVolume,
			RecentlyUpdated:   recentlyUpdated,
			AvgFlipEfficiency: avgEfficiency,
		},
	}
}

// FilteredItemData represents item data with filtered fields for analysis
type FilteredItemData struct {
	ItemID            int        `json:"item_id"`
	Name              string     `json:"name"`
	InstaBuyPrice     *int       `json:"insta_buy_price"`  // What you can SELL at
	InstaSellPrice    *int       `json:"insta_sell_price"` // What you can BUY at
	LastInstaBuyTime  *time.Time `json:"last_insta_buy_time"`
	LastInstaSellTime *time.Time `json:"last_insta_sell_time"`
	BuyLimit          int        `json:"buy_limit"`

	// Derived columns (computed after loading)
	MarginGP  int     `json:"margin_gp"`
	MarginPct float64 `json:"margin_pct"`

	// Volume metrics
	InstaBuyVolume20m    *float64 `json:"insta_buy_volume_20m,omitempty"`
	InstaSellVolume20m   *float64 `json:"insta_sell_volume_20m,omitempty"`
	AvgInstaBuyPrice20m  *float64 `json:"avg_insta_buy_price_20m,omitempty"`
	AvgInstaSellPrice20m *float64 `json:"avg_insta_sell_price_20m,omitempty"`
	AvgMarginGP20m       *float64 `json:"avg_margin_gp_20m,omitempty"`

	InstaBuyVolume1h    *float64 `json:"insta_buy_volume_1h,omitempty"`
	InstaSellVolume1h   *float64 `json:"insta_sell_volume_1h,omitempty"`
	AvgInstaBuyPrice1h  *float64 `json:"avg_insta_buy_price_1h,omitempty"`
	AvgInstaSellPrice1h *float64 `json:"avg_insta_sell_price_1h,omitempty"`
	AvgMarginGP1h       *float64 `json:"avg_margin_gp_1h,omitempty"`

	InstaBuyVolume24h    *float64 `json:"insta_buy_volume_24h,omitempty"`
	InstaSellVolume24h   *float64 `json:"insta_sell_volume_24h,omitempty"`
	AvgInstaBuyPrice24h  *float64 `json:"avg_insta_buy_price_24h,omitempty"`
	AvgInstaSellPrice24h *float64 `json:"avg_insta_sell_price_24h,omitempty"`
	AvgMarginGP24h       *float64 `json:"avg_margin_gp_24h,omitempty"`

	// Trend analysis
	InstaSellPriceTrend1h  *string `json:"insta_sell_price_trend_1h,omitempty"`
	InstaBuyPriceTrend1h   *string `json:"insta_buy_price_trend_1h,omitempty"`
	InstaSellPriceTrend24h *string `json:"insta_sell_price_trend_24h,omitempty"`
	InstaBuyPriceTrend24h  *string `json:"insta_buy_price_trend_24h,omitempty"`
	InstaSellPriceTrend1w  *string `json:"insta_sell_price_trend_1w,omitempty"`
	InstaBuyPriceTrend1w   *string `json:"insta_buy_price_trend_1w,omitempty"`
	InstaSellPriceTrend1m  *string `json:"insta_sell_price_trend_1m,omitempty"`
	InstaBuyPriceTrend1m   *string `json:"insta_buy_price_trend_1m,omitempty"`
}

// FormatItemsForAnalysis creates a compressed array representation of items for LLM analysis
func FormatItemsForAnalysis(items []osrs.ItemData, maxItems int) string {
	if len(items) == 0 {
		return `Schema: []
Data: []
Message: No items available for analysis.`
	}

	if maxItems > 0 && len(items) > maxItems {
		items = items[:maxItems]
	}

	// Define schema based on FilteredItemData JSON tags and types
	schema := []string{
		"item_id",
		"name",
		"insta_buy_price",
		"insta_sell_price",
		"last_insta_buy_time",
		"last_insta_sell_time",
		"buy_limit",
		"margin_gp",
		"margin_pct",
		"insta_buy_volume_20m",
		"insta_sell_volume_20m",
		"avg_insta_buy_price_20m",
		"avg_insta_sell_price_20m",
		"avg_margin_gp_20m",
		"insta_buy_volume_1h",
		"insta_sell_volume_1h",
		"avg_insta_buy_price_1h",
		"avg_insta_sell_price_1h",
		"avg_margin_gp_1h",
		"insta_buy_volume_24h",
		"insta_sell_volume_24h",
		"avg_insta_buy_price_24h",
		"avg_insta_sell_price_24h",
		"avg_margin_gp_24h",
		"insta_sell_price_trend_1h",
		"insta_buy_price_trend_1h",
		"insta_sell_price_trend_24h",
		"insta_buy_price_trend_24h",
		"insta_sell_price_trend_1w",
		"insta_buy_price_trend_1w",
		"insta_sell_price_trend_1m",
		"insta_buy_price_trend_1m",
	}

	// Build data array
	data := make([][]interface{}, len(items))
	for i, item := range items {
		// Convert time pointers to strings for serialization
		var lastBuyTime, lastSellTime interface{}
		if item.LastInstaBuyTime != nil {
			lastBuyTime = item.LastInstaBuyTime.Format(time.RFC3339)
		}
		if item.LastInstaSellTime != nil {
			lastSellTime = item.LastInstaSellTime.Format(time.RFC3339)
		}

		data[i] = []interface{}{
			item.ItemID,
			item.Name,
			item.InstaBuyPrice,
			item.InstaSellPrice,
			lastBuyTime,
			lastSellTime,
			item.BuyLimit,
			item.MarginGP,
			item.MarginPct,
			item.InstaBuyVolume20m,
			item.InstaSellVolume20m,
			item.AvgInstaBuyPrice20m,
			item.AvgInstaSellPrice20m,
			item.AvgMarginGP20m,
			item.InstaBuyVolume1h,
			item.InstaSellVolume1h,
			item.AvgInstaBuyPrice1h,
			item.AvgInstaSellPrice1h,
			item.AvgMarginGP1h,
			item.InstaBuyVolume24h,
			item.InstaSellVolume24h,
			item.AvgInstaBuyPrice24h,
			item.AvgInstaSellPrice24h,
			item.AvgMarginGP24h,
			item.InstaSellPriceTrend1h,
			item.InstaBuyPriceTrend1h,
			item.InstaSellPriceTrend24h,
			item.InstaBuyPriceTrend24h,
			item.InstaSellPriceTrend1w,
			item.InstaBuyPriceTrend1w,
			item.InstaSellPriceTrend1m,
			item.InstaBuyPriceTrend1m,
		}
	}

	// Marshal schema and data arrays
	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return fmt.Sprintf("Error marshaling schema: %v", err)
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("Error marshaling data: %v", err)
	}

	// Format in the requested compressed format
	return fmt.Sprintf(`
Schema: %s
Data: %s`, string(schemaJSON), string(dataJSON))
}

// FormatItemsForAnalysisV2 creates a semantic JSON representation optimized for LLM understanding
func FormatItemsForAnalysisV2(items []osrs.ItemData, maxItems int) string {
	if len(items) == 0 {
		return `{"trading_opportunities": [], "message": "No items available for analysis."}`
	}

	if maxItems > 0 && len(items) > maxItems {
		items = items[:maxItems]
	}

	type TradingOpportunity struct {
		ItemID int    `json:"item_id"`
		Name   string `json:"name"`

		// Prices with clear semantics (GP values)
		BuyAtPrice  *int `json:"buy_at_price"`  // insta_sell_price - what you pay to buy instantly
		SellAtPrice *int `json:"sell_at_price"` // insta_buy_price - what you get selling instantly
		BuyLimit    int  `json:"buy_limit"`

		// Profit metrics (GP values)
		MarginGP  int     `json:"margin_gp"`
		MarginPct float64 `json:"margin_pct"`

		// Volume metrics (transaction counts, not GP values)
		VolumeMetrics map[string]interface{} `json:"volume_metrics,omitempty"`

		// Price averages over time (GP values)
		PriceAverages map[string]interface{} `json:"price_averages,omitempty"`

		// Trend indicators
		TrendSignals map[string]*string `json:"trend_signals,omitempty"`

		// Timing info
		LastUpdated map[string]interface{} `json:"last_updated,omitempty"`
	}

	opportunities := make([]TradingOpportunity, len(items))
	for i, item := range items {
		volumeMetrics := make(map[string]interface{})
		priceAverages := make(map[string]interface{})
		trendSignals := make(map[string]*string)
		lastUpdated := make(map[string]interface{})

		// Add volume data with clear labels (transaction counts)
		if item.InstaBuyVolume20m != nil && item.InstaSellVolume20m != nil {
			volumeMetrics["transactions_20m"] = map[string]float64{
				"buy_orders":  *item.InstaBuyVolume20m,
				"sell_orders": *item.InstaSellVolume20m,
				"total":       *item.InstaBuyVolume20m + *item.InstaSellVolume20m,
			}
		}

		if item.InstaBuyVolume1h != nil && item.InstaSellVolume1h != nil {
			volumeMetrics["transactions_1h"] = map[string]float64{
				"buy_orders":  *item.InstaBuyVolume1h,
				"sell_orders": *item.InstaSellVolume1h,
				"total":       *item.InstaBuyVolume1h + *item.InstaSellVolume1h,
			}
		}

		if item.InstaBuyVolume24h != nil && item.InstaSellVolume24h != nil {
			volumeMetrics["transactions_24h"] = map[string]float64{
				"buy_orders":  *item.InstaBuyVolume24h,
				"sell_orders": *item.InstaSellVolume24h,
				"total":       *item.InstaBuyVolume24h + *item.InstaSellVolume24h,
			}
		}

		// Add price averages (GP values)
		if item.AvgInstaBuyPrice20m != nil && item.AvgInstaSellPrice20m != nil {
			priceAverages["avg_prices_20m"] = map[string]float64{
				"avg_sell_at_price": *item.AvgInstaBuyPrice20m,  // what you get selling
				"avg_buy_at_price":  *item.AvgInstaSellPrice20m, // what you pay buying
			}
			if item.AvgMarginGP20m != nil {
				priceAverages["avg_prices_20m"].(map[string]float64)["avg_margin_gp"] = *item.AvgMarginGP20m
			}
		}

		if item.AvgInstaBuyPrice1h != nil && item.AvgInstaSellPrice1h != nil {
			priceAverages["avg_prices_1h"] = map[string]float64{
				"avg_sell_at_price": *item.AvgInstaBuyPrice1h,  // what you get selling
				"avg_buy_at_price":  *item.AvgInstaSellPrice1h, // what you pay buying
			}
			if item.AvgMarginGP1h != nil {
				priceAverages["avg_prices_1h"].(map[string]float64)["avg_margin_gp"] = *item.AvgMarginGP1h
			}
		}

		if item.AvgInstaBuyPrice24h != nil && item.AvgInstaSellPrice24h != nil {
			priceAverages["avg_prices_24h"] = map[string]float64{
				"avg_sell_at_price": *item.AvgInstaBuyPrice24h,  // what you get selling
				"avg_buy_at_price":  *item.AvgInstaSellPrice24h, // what you pay buying
			}
			if item.AvgMarginGP24h != nil {
				priceAverages["avg_prices_24h"].(map[string]float64)["avg_margin_gp"] = *item.AvgMarginGP24h
			}
		}

		// Trend signals with clear timeframes
		if item.InstaBuyPriceTrend1h != nil {
			trendSignals["sell_price_trend_1h"] = item.InstaBuyPriceTrend1h // price you get selling
		}
		if item.InstaSellPriceTrend1h != nil {
			trendSignals["buy_price_trend_1h"] = item.InstaSellPriceTrend1h // price you pay buying
		}
		if item.InstaBuyPriceTrend24h != nil {
			trendSignals["sell_price_trend_24h"] = item.InstaBuyPriceTrend24h
		}
		if item.InstaSellPriceTrend24h != nil {
			trendSignals["buy_price_trend_24h"] = item.InstaSellPriceTrend24h
		}
		if item.InstaBuyPriceTrend1w != nil {
			trendSignals["sell_price_trend_1w"] = item.InstaBuyPriceTrend1w
		}
		if item.InstaSellPriceTrend1w != nil {
			trendSignals["buy_price_trend_1w"] = item.InstaSellPriceTrend1w
		}
		if item.InstaBuyPriceTrend1m != nil {
			trendSignals["sell_price_trend_1m"] = item.InstaBuyPriceTrend1m
		}
		if item.InstaSellPriceTrend1m != nil {
			trendSignals["buy_price_trend_1m"] = item.InstaSellPriceTrend1m
		}

		// Last updated timestamps
		if item.LastInstaBuyTime != nil {
			lastUpdated["sell_price_updated"] = item.LastInstaBuyTime.Format(time.RFC3339) // when sell price was last updated
		}
		if item.LastInstaSellTime != nil {
			lastUpdated["buy_price_updated"] = item.LastInstaSellTime.Format(time.RFC3339) // when buy price was last updated
		}

		opportunities[i] = TradingOpportunity{
			ItemID:        item.ItemID,
			Name:          item.Name,
			BuyAtPrice:    item.InstaSellPrice, // Clear semantic mapping
			SellAtPrice:   item.InstaBuyPrice,  // Clear semantic mapping
			BuyLimit:      item.BuyLimit,
			MarginGP:      item.MarginGP,
			MarginPct:     item.MarginPct,
			VolumeMetrics: volumeMetrics,
			PriceAverages: priceAverages,
			TrendSignals:  trendSignals,
			LastUpdated:   lastUpdated,
		}
	}

	result := map[string]interface{}{
		"trading_opportunities": opportunities,
		"context": map[string]string{
			"volume_note": "Volume metrics show transaction counts, not GP values",
			"price_note":  "Buy at 'buy_at_price' (insta_sell_price), sell at 'sell_at_price' (insta_buy_price)",
			"trend_note":  "Trends: 'increasing', 'decreasing', or 'flat'",
			"margin_calc": "margin_gp = sell_at_price - buy_at_price",
		},
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshaling trading opportunities: %v", err)
	}
	return string(data)
}

// ToJSON converts the analysis request to JSON
func (r TradingAnalysisRequest) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
