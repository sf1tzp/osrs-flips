package llm

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
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
// FIXME - incorrect margins displayed when no transactions occured in a window, eg:
//
//	"transactions_20m": {
//	  "at_target_buy_price": "0.00",
//	  "at_target_sell_price": "2.00"
//	}
//	"avg_prices_20m": {
//	  "margin_gp": "4673005.00",
//	  "target_buy_price": "0.00",
//	  "target_sell_price": "4673005.00"
//	},
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

		// Prices using OSRS terminology (GP values)
		LastSellPrice *int `json:"last_sell_price"` // Price where buy orders get filled instantly
		LastBuyPrice  *int `json:"last_buy_price"`  // Price where sell orders get filled instantly

		// Profit metrics (GP values)
		MarginGP  int    `json:"margin_gp"`
		MarginPct string `json:"margin_pct"`

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

		// Add volume data with OSRS terminology (transaction counts)
		if item.InstaBuyVolume20m != nil && item.InstaSellVolume20m != nil {
			volumeMetrics["transactions_20m"] = map[string]string{
				"at_target_sell_price": fmt.Sprintf("%.0f", *item.InstaBuyVolume20m),
				"at_target_buy_price":  fmt.Sprintf("%.0f", *item.InstaSellVolume20m),
			}
		}

		if item.InstaBuyVolume1h != nil && item.InstaSellVolume1h != nil {
			volumeMetrics["transactions_1h"] = map[string]string{
				"at_target_sell_price": fmt.Sprintf("%.0f", *item.InstaBuyVolume1h),
				"at_target_buy_price":  fmt.Sprintf("%.0f", *item.InstaSellVolume1h),
			}
		}

		if item.InstaBuyVolume24h != nil && item.InstaSellVolume24h != nil {
			volumeMetrics["transactions_24h"] = map[string]string{
				"at_target_sell_price": fmt.Sprintf("%.0f", *item.InstaBuyVolume24h),
				"at_target_buy_price":  fmt.Sprintf("%.0f", *item.InstaSellVolume24h),
			}
		}

		// Add price averages using OSRS terminology (GP values)
		if (item.AvgInstaBuyPrice20m != nil && item.AvgInstaSellPrice20m != nil) && (*item.AvgInstaBuyPrice20m != 0.0 && *item.AvgInstaSellPrice20m != 0.0) {
			priceAverages["avg_prices_20m"] = map[string]string{
				"target_sell_price": fmt.Sprintf("%.0f", *item.AvgInstaBuyPrice20m),
				"target_buy_price":  fmt.Sprintf("%.0f", *item.AvgInstaSellPrice20m),
			}
			if item.AvgMarginGP20m != nil {
				priceAverages["avg_prices_20m"].(map[string]string)["margin_gp"] = fmt.Sprintf("%.2f", *item.AvgMarginGP20m)
			}
		}

		if (item.AvgInstaBuyPrice1h != nil && item.AvgInstaSellPrice1h != nil) && (*item.AvgInstaBuyPrice1h != 0.0 && *item.AvgInstaSellPrice1h != 0.0) {
			priceAverages["avg_prices_1h"] = map[string]string{
				"target_sell_price": fmt.Sprintf("%.0f", *item.AvgInstaBuyPrice1h),
				"target_buy_price":  fmt.Sprintf("%.0f", *item.AvgInstaSellPrice1h),
			}
			if item.AvgMarginGP1h != nil {
				priceAverages["avg_prices_1h"].(map[string]string)["margin_gp"] = fmt.Sprintf("%.2f", *item.AvgMarginGP1h)
			}
		}

		if (item.AvgInstaBuyPrice24h != nil && item.AvgInstaSellPrice24h != nil) && (*item.AvgInstaBuyPrice24h != 0.0 && *item.AvgInstaSellPrice24h != 0.0) {
			priceAverages["avg_prices_24h"] = map[string]string{
				"target_sell_price": fmt.Sprintf("%.0f", *item.AvgInstaBuyPrice24h),
				"target_buy_price":  fmt.Sprintf("%.0f", *item.AvgInstaSellPrice24h),
			}
			if item.AvgMarginGP24h != nil {
				priceAverages["avg_prices_24h"].(map[string]string)["margin_gp"] = fmt.Sprintf("%.2f", *item.AvgMarginGP24h)
			}
		}

		// Trend signals using exact OSRS field names
		if item.InstaBuyPriceTrend1h != nil {
			trendSignals["target_sell_price_trend_1h"] = item.InstaBuyPriceTrend1h
		}
		if item.InstaSellPriceTrend1h != nil {
			trendSignals["target_buy_price_trend_1h"] = item.InstaSellPriceTrend1h
		}
		if item.InstaBuyPriceTrend24h != nil {
			trendSignals["target_sell_price_trend_24h"] = item.InstaBuyPriceTrend24h
		}
		if item.InstaSellPriceTrend24h != nil {
			trendSignals["target_buy_price_trend_24h"] = item.InstaSellPriceTrend24h
		}
		if item.InstaBuyPriceTrend1w != nil {
			trendSignals["target_sell_price_trend_1w"] = item.InstaBuyPriceTrend1w
		}
		if item.InstaSellPriceTrend1w != nil {
			trendSignals["target_buy_price_trend_1w"] = item.InstaSellPriceTrend1w
		}
		if item.InstaBuyPriceTrend1m != nil {
			trendSignals["target_sell_price_trend_1month"] = item.InstaBuyPriceTrend1m
		}
		if item.InstaSellPriceTrend1m != nil {
			trendSignals["target_buy_price_trend_1month"] = item.InstaSellPriceTrend1m
		}

		// Last updated timestamps using OSRS field names
		if item.LastInstaBuyTime != nil {
			lastUpdated["last_target_sell_price_time"] = item.LastInstaBuyTime.Format(time.RFC3339)
		}
		if item.LastInstaSellTime != nil {
			lastUpdated["last_target_buy_price_time"] = item.LastInstaSellTime.Format(time.RFC3339)
		}

		afterTax := math.Floor(0.98 * float64(item.MarginGP))
		afterTaxPct := item.MarginPct - 2

		opportunities[i] = TradingOpportunity{
			ItemID:        item.ItemID,
			Name:          item.Name,
			LastSellPrice: item.InstaBuyPrice,
			LastBuyPrice:  item.InstaSellPrice,
			MarginGP:      int(afterTax),
			MarginPct:     fmt.Sprintf("%.2f", afterTaxPct),
			VolumeMetrics: volumeMetrics,
			PriceAverages: priceAverages,
			TrendSignals:  trendSignals,
			LastUpdated:   lastUpdated,
		}
	}

	// experimental extra context (should not be required, already in the prompt)
	_ = map[string]interface{}{
		"trading_opportunities": opportunities,
		"context": map[string]string{
			"volume_note": "Volume metrics show transaction counts, not GP values",
			"price_note":  "insta_buy_price = price where buy orders get filled instantly; insta_sell_price = price where sell orders get filled instantly",
			"trend_note":  "Trends: 'increasing', 'decreasing', or 'flat'",
			"margin_calc": "margin_gp = insta_buy_price - insta_sell_price (buy low at insta_sell_price, sell high at insta_buy_price)",
			"strategy":    "Buy orders target insta_sell_price (low); Sell orders target insta_buy_price (high)",
		},
	}

	data, err := json.MarshalIndent(opportunities, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshaling trading opportunities: %v", err)
	}

	// TODO: make this configurable or only when run via cli
	// And/or have the bot attach the file to it's message
	// Save to file with timestamp in output/data/ directory
	if false {
		timestamp := time.Now().Format("2006-01-02T15-04-05")
		filename := fmt.Sprintf("output/data/analysis_v2-%s.json", timestamp)

		// Create directory if it doesn't exist
		if err := os.MkdirAll("output/data", 0755); err != nil {
			fmt.Printf("Warning: Could not create directory: %v\n", err)
		}

		if err := os.WriteFile(filename, data, 0644); err != nil {
			// Log error but don't fail the function
			fmt.Printf("Warning: Could not save analysis to %s: %v\n", filename, err)
		}
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
