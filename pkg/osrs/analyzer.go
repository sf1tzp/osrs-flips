package osrs

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Analyzer is the main class equivalent to OSRSItemFilter in Python
type Analyzer struct {
	client *Client
	items  []ItemData
}

// NewAnalyzer creates a new OSRS analyzer instance
func NewAnalyzer(userAgent string) *Analyzer {
	return &Analyzer{
		client: NewClient(userAgent),
		items:  make([]ItemData, 0),
	}
}

// LoadData fetches and merges item mappings with latest prices
// Equivalent to load_data method in Python
func (a *Analyzer) LoadData(ctx context.Context, forceReload bool) error {
	if !forceReload && len(a.items) > 0 {
		fmt.Println("Data already loaded. Use forceReload=true to refresh.")
		return nil
	}

	fmt.Println("Loading data for filtering...")

	// Get item mappings
	mappings, err := a.client.GetItemMapping(ctx)
	if err != nil {
		return fmt.Errorf("getting item mappings: %w", err)
	}

	// Get latest prices
	prices, err := a.client.GetLatestPrices(ctx, nil)
	if err != nil {
		return fmt.Errorf("getting latest prices: %w", err)
	}

	// Merge data (equivalent to merge_prices_with_items)
	a.items = a.mergePricesWithItems(prices, mappings)

	// Compute derived columns
	a.computeDerivedColumns()

	fmt.Printf("✅ Loaded %d items with price data\n", len(a.items))
	return nil
}

// mergePricesWithItems combines price data with item metadata
func (a *Analyzer) mergePricesWithItems(prices *LatestPricesResponse, mappings []ItemMapping) []ItemData {
	// Create mapping lookup for O(1) access
	itemMap := make(map[int]ItemMapping)
	for _, item := range mappings {
		itemMap[item.ID] = item
	}

	var items []ItemData
	for itemIDStr, priceInfo := range prices.Data {
		itemID, err := strconv.Atoi(itemIDStr)
		if err != nil {
			continue // Skip invalid item IDs
		}

		mapping, exists := itemMap[itemID]
		if !exists {
			continue // Skip items without mapping data
		}

		item := ItemData{
			ItemID:         itemID,
			Name:           mapping.Name,
			InstaBuyPrice:  priceInfo.High, // Counter-intuitive: High = what you can sell at
			InstaSellPrice: priceInfo.Low,  // Counter-intuitive: Low = what you can buy at
			BuyLimit:       mapping.BuyLimit,
			Members:        mapping.Members,
		}

		// Convert timestamps (handle nil pointers)
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

// computeDerivedColumns calculates margin_gp, margin_pct, and flip_efficiency
// Equivalent to compute_derived_columns method in Python
func (a *Analyzer) computeDerivedColumns() {
	for i := range a.items {
		item := &a.items[i]

		// Calculate margin_gp and margin_pct
		if item.InstaBuyPrice != nil && item.InstaSellPrice != nil {
			marginGP := *item.InstaBuyPrice - *item.InstaSellPrice
			item.MarginGP = marginGP

			if *item.InstaSellPrice > 0 {
				item.MarginPct = (float64(marginGP) / float64(*item.InstaSellPrice)) * 100
			}

			// Calculate flip_efficiency (margin_gp * buy_limit)
			if item.BuyLimit > 0 {
				item.FlipEfficiency = float64(marginGP * item.BuyLimit)
			}
		}
	}

	fmt.Println("✅ Computed derived columns (margin_gp, margin_pct, flip_efficiency)")
}

// HasData checks if data has been loaded
func (a *Analyzer) HasData() bool {
	return len(a.items) > 0
}

// GetData returns a copy of the current items
func (a *Analyzer) GetData() []ItemData {
	result := make([]ItemData, len(a.items))
	copy(result, a.items)
	return result
}

// GetItemsWithVolume returns items that have volume data loaded, filtered by the provided item IDs
func (a *Analyzer) GetItemsWithVolume(itemIDs []int) []ItemData {
	// Create a map for fast lookup
	idMap := make(map[int]bool)
	for _, id := range itemIDs {
		idMap[id] = true
	}

	result := make([]ItemData, 0)
	for _, item := range a.items {
		if idMap[item.ItemID] {
			result = append(result, item)
		}
	}
	return result
}

// ApplyFilter applies filtering criteria and returns filtered results
// Equivalent to apply_filter method in Python
func (a *Analyzer) ApplyFilter(opts FilterOptions, verbose bool) ([]ItemData, error) {
	if !a.HasData() {
		return nil, fmt.Errorf("no data available for filtering. Use LoadData() first")
	}

	// Work with a copy to preserve original data
	filtered := make([]ItemData, 0, len(a.items))

	if verbose {
		fmt.Printf("Starting with %d items with price data\n", len(a.items))
	}

	// Apply filters
	for _, item := range a.items {
		if a.passesFilter(item, opts) {
			filtered = append(filtered, item)
		}
	}

	if verbose {
		fmt.Printf("After filtering: %d items remain\n", len(filtered))
	}

	// Apply sorting
	if opts.SortBy != "" {
		a.sortItems(filtered, opts.SortBy, opts.SortDesc)
		if verbose {
			fmt.Printf("Sorted by %s (desc=%t)\n", opts.SortBy, opts.SortDesc)
		}
	}

	// Apply limit
	if opts.Limit > 0 && len(filtered) > opts.Limit {
		filtered = filtered[:opts.Limit]
		if verbose {
			fmt.Printf("Limited to %d items\n", opts.Limit)
		}
	}

	return filtered, nil
}

// ApplyPrimaryFilter applies price-based filters only (before volume data is loaded)
func (a *Analyzer) ApplyPrimaryFilter(opts FilterOptions, verbose bool) ([]ItemData, error) {
	if !a.HasData() {
		return nil, fmt.Errorf("no data available for filtering. Use LoadData() first")
	}

	// Create options with only primary filters (no volume filters)
	primaryOpts := FilterOptions{
		BuyLimitMin:         opts.BuyLimitMin,
		BuyLimitMax:         opts.BuyLimitMax,
		InstaBuyPriceMin:    opts.InstaBuyPriceMin,
		InstaBuyPriceMax:    opts.InstaBuyPriceMax,
		InstaSellPriceMin:   opts.InstaSellPriceMin,
		InstaSellPriceMax:   opts.InstaSellPriceMax,
		MarginMin:           opts.MarginMin,
		MarginMax:           opts.MarginMax,
		MarginPctMin:        opts.MarginPctMin,
		MarginPctMax:        opts.MarginPctMax,
		MembersOnly:         opts.MembersOnly,
		MaxHoursSinceUpdate: opts.MaxHoursSinceUpdate,
		NameContains:        opts.NameContains,
		ExcludeItems:        opts.ExcludeItems,
		SortBy:              opts.SortBy,
		SortDesc:            opts.SortDesc,
		Limit:               opts.Limit,
		// Volume filters excluded: Volume1hMin, Volume24hMin
	}

	if verbose {
		fmt.Printf("Applying primary filters (before volume data)...\n")
	}

	return a.ApplyFilter(primaryOpts, verbose)
}

// ApplySecondaryFilter applies volume-based filters to items that already have volume data
func (a *Analyzer) ApplySecondaryFilter(items []ItemData, opts FilterOptions, verbose bool) ([]ItemData, error) {
	// Apply volume-based filters
	filtered := make([]ItemData, 0, len(items))

	if verbose {
		fmt.Printf("Applying secondary filters (volume-based) to %d items...\n", len(items))
	}

	for _, item := range items {
		if a.passesVolumeFilters(item, opts) {
			filtered = append(filtered, item)
		}
	}

	if verbose {
		fmt.Printf("After volume filtering: %d items remain\n", len(filtered))
	}

	// Apply sorting
	if opts.SortBy != "" {
		a.sortItems(filtered, "insta_buy_volume_20m", opts.SortDesc)
		if verbose {
			fmt.Printf("Sorted by insta_buy_volume_20m (desc=%t)\n", opts.SortDesc)
		}
	}

	// Apply limit
	if opts.Limit > 0 && len(filtered) > opts.Limit {
		filtered = filtered[:opts.Limit]
		if verbose {
			fmt.Printf("Limited to %d items\n", opts.Limit)
		}
	}

	return filtered, nil
}

// passesVolumeFilters checks if an item passes volume-based filter criteria
func (a *Analyzer) passesVolumeFilters(item ItemData, opts FilterOptions) bool {
	// Volume filters - both buy and sell volumes must individually meet the threshold
	if opts.Volume1hMin != nil {
		// Both buy and sell volumes must be present and >= threshold
		if item.InstaBuyVolume1h == nil || item.InstaSellVolume1h == nil {
			return false
		}

		thresholdFloat := float64(*opts.Volume1hMin)
		if *item.InstaBuyVolume1h < thresholdFloat || *item.InstaSellVolume1h < thresholdFloat {
			return false
		}
	}

	if opts.Volume24hMin != nil {
		// Both buy and sell volumes must be present and >= threshold
		if item.InstaBuyVolume24h == nil || item.InstaSellVolume24h == nil {
			return false
		}

		thresholdFloat := float64(*opts.Volume24hMin)
		if *item.InstaBuyVolume24h < thresholdFloat || *item.InstaSellVolume24h < thresholdFloat {
			return false
		}
	}

	return true
}

// passesFilter checks if an item passes all filter criteria
func (a *Analyzer) passesFilter(item ItemData, opts FilterOptions) bool {
	// Buy limit filters
	if opts.BuyLimitMin != nil && item.BuyLimit < *opts.BuyLimitMin {
		return false
	}
	if opts.BuyLimitMax != nil && item.BuyLimit > *opts.BuyLimitMax {
		return false
	}

	// Price filters
	if opts.InstaBuyPriceMin != nil && (item.InstaBuyPrice == nil || *item.InstaBuyPrice < *opts.InstaBuyPriceMin) {
		return false
	}
	if opts.InstaBuyPriceMax != nil && (item.InstaBuyPrice == nil || *item.InstaBuyPrice > *opts.InstaBuyPriceMax) {
		return false
	}
	if opts.InstaSellPriceMin != nil && (item.InstaSellPrice == nil || *item.InstaSellPrice < *opts.InstaSellPriceMin) {
		return false
	}
	if opts.InstaSellPriceMax != nil && (item.InstaSellPrice == nil || *item.InstaSellPrice > *opts.InstaSellPriceMax) {
		return false
	}

	// Margin filters
	if opts.MarginMin != nil && item.MarginGP < *opts.MarginMin {
		return false
	}
	if opts.MarginMax != nil && item.MarginGP > *opts.MarginMax {
		return false
	}
	if opts.MarginPctMin != nil && item.MarginPct < *opts.MarginPctMin {
		return false
	}
	if opts.MarginPctMax != nil && item.MarginPct > *opts.MarginPctMax {
		return false
	}

	// Volume filters (only if volume data is loaded)
	if opts.Volume1hMin != nil {
		if item.InstaBuyVolume1h == nil || item.InstaSellVolume1h == nil ||
			*item.InstaBuyVolume1h < float64(*opts.Volume1hMin) ||
			*item.InstaSellVolume1h < float64(*opts.Volume1hMin) {
			return false
		}
	}
	if opts.Volume24hMin != nil {
		if item.InstaBuyVolume24h == nil || item.InstaSellVolume24h == nil ||
			*item.InstaBuyVolume24h < float64(*opts.Volume24hMin) ||
			*item.InstaSellVolume24h < float64(*opts.Volume24hMin) {
			return false
		}
	}

	// Members filter
	if opts.MembersOnly != nil && item.Members != *opts.MembersOnly {
		return false
	}

	// Time since update filter
	if opts.MaxHoursSinceUpdate != nil {
		maxDuration := time.Duration(*opts.MaxHoursSinceUpdate * float64(time.Hour))
		now := time.Now().UTC()

		if item.LastInstaBuyTime != nil {
			if now.Sub(*item.LastInstaBuyTime) > maxDuration {
				return false
			}
		}
		if item.LastInstaSellTime != nil {
			if now.Sub(*item.LastInstaSellTime) > maxDuration {
				return false
			}
		}
	}

	// Name contains filter
	if opts.NameContains != nil {
		if !strings.Contains(strings.ToLower(item.Name), strings.ToLower(*opts.NameContains)) {
			return false
		}
	}

	// Exclude items filter
	if len(opts.ExcludeItems) > 0 {
		itemNameLower := strings.ToLower(item.Name)
		for _, exclude := range opts.ExcludeItems {
			if strings.Contains(itemNameLower, strings.ToLower(exclude)) {
				return false
			}
		}
	}

	return true
}

// sortItems sorts the filtered items by the specified field
func (a *Analyzer) sortItems(items []ItemData, sortBy string, desc bool) {
	sort.Slice(items, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "margin_gp":
			less = items[i].MarginGP < items[j].MarginGP
		case "margin_pct":
			less = items[i].MarginPct < items[j].MarginPct
		case "flip_efficiency":
			less = items[i].FlipEfficiency < items[j].FlipEfficiency
		case "insta_buy_price":
			less = a.compareIntPtr(items[i].InstaBuyPrice, items[j].InstaBuyPrice)
		case "insta_sell_price":
			less = a.compareIntPtr(items[i].InstaSellPrice, items[j].InstaSellPrice)
		case "buy_limit":
			less = items[i].BuyLimit < items[j].BuyLimit
		case "name":
			less = items[i].Name < items[j].Name
		case "last_insta_buy_time":
			less = a.compareTimePtr(items[i].LastInstaBuyTime, items[j].LastInstaBuyTime)
		case "last_insta_sell_time":
			less = a.compareTimePtr(items[i].LastInstaSellTime, items[j].LastInstaSellTime)
		default:
			// Default to sorting by margin_gp
			less = items[i].MarginGP < items[j].MarginGP
		}

		if desc {
			return !less
		}
		return less
	})
}

// Helper functions for comparing pointer values
func (a *Analyzer) compareIntPtr(a1, a2 *int) bool {
	if a1 == nil && a2 == nil {
		return false
	}
	if a1 == nil {
		return true
	}
	if a2 == nil {
		return false
	}
	return *a1 < *a2
}

func (a *Analyzer) compareTimePtr(t1, t2 *time.Time) bool {
	if t1 == nil && t2 == nil {
		return false
	}
	if t1 == nil {
		return true
	}
	if t2 == nil {
		return false
	}
	return t1.Before(*t2)
}
