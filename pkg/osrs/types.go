package osrs

import "time"

// LatestPricesResponse matches the RuneScape Wiki API response structure
type LatestPricesResponse struct {
	Data map[string]PriceInfo `json:"data"`
}

/*

Please note:

The OSRS trading API data is counterintuitive to normal trading:

`low` = insta_sell_price = Price where sell orders get filled instantly
`high` = insta_buy_price = Price where buy orders get filled instantly

(potential) margin_gp = insta_buy_price - insta_sell_price

Trading Strategy
When buying, we target the low insta_sell_price (buy at a low price people are trying to insta-sell at)
When selling, we target the high insta_buy_price (sell at a high price people are trying to insta-buy at)

*/

type PriceInfo struct {
	High     *int `json:"high"`     // insta_buy_price in our terminology
	HighTime *int `json:"highTime"` // last_insta_buy_time
	Low      *int `json:"low"`      // insta_sell_price in our terminology
	LowTime  *int `json:"lowTime"`  // last_insta_sell_time
}

// ItemMapping represents the item metadata from the mapping API
type ItemMapping struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Examine  string `json:"examine"`
	Members  bool   `json:"members"`
	BuyLimit int    `json:"limit"` // Maps to buy_limit column
	Value    int    `json:"value"`
	HighAlch int    `json:"highalch"`
	LowAlch  int    `json:"lowalch"`
	Icon     string `json:"icon"`
}

// ItemData represents our core data structure (equivalent to DataFrame row)
type ItemData struct {
	ItemID            int        `json:"item_id"`
	Name              string     `json:"name"`
	InstaBuyPrice     *int       `json:"insta_buy_price"`
	InstaSellPrice    *int       `json:"insta_sell_price"`
	LastInstaBuyTime  *time.Time `json:"last_insta_buy_time"`
	LastInstaSellTime *time.Time `json:"last_insta_sell_time"`
	BuyLimit          int        `json:"buy_limit"`
	Members           bool       `json:"members"`

	// Derived columns (computed after loading)
	MarginGP       int     `json:"margin_gp"`
	MarginPct      float64 `json:"margin_pct"`
	FlipEfficiency float64 `json:"flip_efficiency"`

	// Volume metrics (added by LoadVolumeData)
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

// VolumeMetrics holds calculated volume and trend data for an item
type VolumeMetrics struct {
	InstaBuyVolume20m    float64
	InstaSellVolume20m   float64
	AvgInstaBuyPrice20m  float64
	AvgInstaSellPrice20m float64
	AvgMarginGP20m       float64

	InstaBuyVolume1h    float64
	InstaSellVolume1h   float64
	AvgInstaBuyPrice1h  float64
	AvgInstaSellPrice1h float64
	AvgMarginGP1h       float64

	InstaBuyVolume24h    float64
	InstaSellVolume24h   float64
	AvgInstaBuyPrice24h  float64
	AvgInstaSellPrice24h float64
	AvgMarginGP24h       float64

	// Trend analysis
	InstaSellPriceTrend1h  string
	InstaBuyPriceTrend1h   string
	InstaSellPriceTrend24h string
	InstaBuyPriceTrend24h  string
	InstaSellPriceTrend1w  string
	InstaBuyPriceTrend1w   string
	InstaSellPriceTrend1m  string
	InstaBuyPriceTrend1m   string
}

// BulkPriceDataPoint represents a single item's data from a bulk price endpoint (/5m, /1h, /24h).
// Same fields as VolumeDataPoint but without per-point timestamp (it's at the top level).
type BulkPriceDataPoint struct {
	AvgHighPrice    *int `json:"avgHighPrice"`
	HighPriceVolume *int `json:"highPriceVolume"`
	AvgLowPrice     *int `json:"avgLowPrice"`
	LowPriceVolume  *int `json:"lowPriceVolume"`
}

// BulkPriceResponse represents the API response from bulk price endpoints (/5m, /1h, /24h).
// The data map is keyed by item ID (as string).
type BulkPriceResponse struct {
	Data      map[string]BulkPriceDataPoint `json:"data"`
	Timestamp int64                         `json:"timestamp"`
}

// FilterOptions defines filtering criteria (equivalent to apply_filter parameters)
type FilterOptions struct {
	BuyLimitMin         *int
	BuyLimitMax         *int
	InstaBuyPriceMin    *int
	InstaBuyPriceMax    *int
	InstaSellPriceMin   *int
	InstaSellPriceMax   *int
	MarginMin           *int
	MarginMax           *int
	MarginPctMin        *float64
	MarginPctMax        *float64
	Volume20mMin        *int
	Volume1hMin         *int
	Volume24hMin        *int
	MembersOnly         *bool
	MaxHoursSinceUpdate *float64
	NameContains        *string
	ExcludeItems        []string
	SortByAfterPrice    string
	SortByAfterVolume   string
	SortDesc            bool
	Limit               int
}
