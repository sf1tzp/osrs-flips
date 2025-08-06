package osrs

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestCalculateTrend(t *testing.T) {
	tests := []struct {
		name     string
		x        []float64
		y        []float64
		expected string
	}{
		{
			name:     "empty data",
			x:        []float64{},
			y:        []float64{},
			expected: "flat",
		},
		{
			name:     "insufficient data points",
			x:        []float64{1, 2},
			y:        []float64{100, 102},
			expected: "flat",
		},
		{
			name:     "mismatched lengths",
			x:        []float64{1, 2, 3},
			y:        []float64{100, 102},
			expected: "flat",
		},
		{
			name:     "clearly increasing trend",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{100, 105, 110, 115, 120}, // 20% increase
			expected: "increasing",
		},
		{
			name:     "clearly decreasing trend",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{120, 115, 110, 105, 100}, // 16.7% decrease
			expected: "decreasing",
		},
		{
			name:     "flat trend - small changes",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{1000, 1005, 1000, 1002, 1001}, // 0.1% change
			expected: "flat",
		},
		{
			name:     "exactly 1% threshold should be flat",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{1000, 1002, 1004, 1006, 1008}, // 0.8% change
			expected: "flat",
		},
		{
			name:     "slightly increasing above 1% threshold",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{1000, 1005, 1008, 1009, 1011}, // 1.1% change
			expected: "increasing",
		},
		{
			name:     "slightly decreasing above 1% threshold",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{1000, 995, 992, 991, 989}, // 1.1% decrease
			expected: "decreasing",
		},
		{
			name:     "volatile but overall flat",
			x:        []float64{1, 2, 3, 4, 5, 6, 7},
			y:        []float64{1000, 1020, 980, 1030, 970, 1010, 1005}, // 0.5% overall change
			expected: "flat",
		},
		{
			name:     "zero starting value",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{0, 1, 2, 3, 4},
			expected: "flat", // Should handle division by zero
		},
		{
			name:     "all same values",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{1000, 1000, 1000, 1000, 1000},
			expected: "flat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateTrend(tt.x, tt.y)
			if result != tt.expected {
				t.Errorf("calculateTrend(%v, %v) = %v, want %v", tt.x, tt.y, result, tt.expected)
			}
		})
	}
}

func TestAverage(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		expected float64
	}{
		{
			name:     "empty slice",
			values:   []float64{},
			expected: 0.0,
		},
		{
			name:     "single value",
			values:   []float64{42.5},
			expected: 42.5,
		},
		{
			name:     "multiple values",
			values:   []float64{1, 2, 3, 4, 5},
			expected: 3.0,
		},
		{
			name:     "negative values",
			values:   []float64{-10, -5, 0, 5, 10},
			expected: 0.0,
		},
		{
			name:     "decimal values",
			values:   []float64{1.5, 2.5, 3.5},
			expected: 2.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := average(tt.values)
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("average(%v) = %v, want %v", tt.values, result, tt.expected)
			}
		})
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		name     string
		a, b     int
		expected int
	}{
		{"a smaller", 5, 10, 5},
		{"b smaller", 10, 5, 5},
		{"equal", 7, 7, 7},
		{"negative values", -5, -2, -5},
		{"zero and positive", 0, 3, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := min(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestRateLimiter(t *testing.T) {
	t.Run("basic rate limiting", func(t *testing.T) {
		// Create a rate limiter that allows 10 requests per second
		rl := NewRateLimiter(10.0)

		// Should have initial tokens
		if rl.tokens != rl.maxTokens {
			t.Errorf("Initial tokens = %d, want %d", rl.tokens, rl.maxTokens)
		}

		ctx := context.Background()

		// First request should pass immediately
		start := time.Now()
		err := rl.Wait(ctx)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("First request failed: %v", err)
		}

		if elapsed > 10*time.Millisecond {
			t.Errorf("First request took too long: %v", elapsed)
		}
	})

	t.Run("rate limiting with context cancellation", func(t *testing.T) {
		rl := NewRateLimiter(0.5) // Very slow: 1 request per 2 seconds

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		// First request consumes the initial token
		err := rl.Wait(ctx)
		if err != nil {
			t.Errorf("First request failed: %v", err)
		}

		// Second request should timeout due to context cancellation
		err = rl.Wait(ctx)
		if err != context.DeadlineExceeded {
			t.Errorf("Expected context deadline exceeded, got: %v", err)
		}
	})
}

func TestProcessTimeseriesData(t *testing.T) {
	analyzer := &Analyzer{}

	t.Run("empty data", func(t *testing.T) {
		data5m := map[string]interface{}{}
		data24h := map[string]interface{}{}

		metrics := analyzer.processTimeseriesData(data5m, data24h)

		// Should return zero values
		if metrics.InstaBuyVolume20m != 0 {
			t.Errorf("Expected zero InstaBuyVolume20m, got %f", metrics.InstaBuyVolume20m)
		}
		if metrics.InstaBuyPriceTrend1h != "" {
			t.Errorf("Expected empty trend, got %s", metrics.InstaBuyPriceTrend1h)
		}
	})

	t.Run("valid data structure", func(t *testing.T) {
		now := time.Now().Unix()

		// Create mock 5m data with recent timestamps
		data5m := map[string]interface{}{
			"data": []interface{}{
				map[string]interface{}{
					"timestamp":       float64(now - 300), // 5 minutes ago
					"avgHighPrice":    1000.0,
					"avgLowPrice":     950.0,
					"highPriceVolume": 100.0,
					"lowPriceVolume":  150.0,
				},
				map[string]interface{}{
					"timestamp":       float64(now - 600), // 10 minutes ago
					"avgHighPrice":    1010.0,
					"avgLowPrice":     960.0,
					"highPriceVolume": 120.0,
					"lowPriceVolume":  180.0,
				},
			},
		}

		data24h := map[string]interface{}{
			"data": []interface{}{
				map[string]interface{}{
					"timestamp":       float64(now - 3600), // 1 hour ago
					"avgHighPrice":    980.0,
					"avgLowPrice":     930.0,
					"highPriceVolume": 200.0,
					"lowPriceVolume":  250.0,
				},
			},
		}

		metrics := analyzer.processTimeseriesData(data5m, data24h)

		// Should have processed the data
		if metrics.InstaBuyVolume20m <= 0 {
			t.Errorf("Expected positive InstaBuyVolume20m, got %f", metrics.InstaBuyVolume20m)
		}
		if metrics.AvgInstaBuyPrice20m <= 0 {
			t.Errorf("Expected positive AvgInstaBuyPrice20m, got %f", metrics.AvgInstaBuyPrice20m)
		}
	})
}

func TestGetTopItemIDs(t *testing.T) {
	// Create a mock analyzer with test data
	buyPrice1, sellPrice1 := 1000, 950
	buyPrice2, sellPrice2 := 2000, 1800
	buyPrice3, sellPrice3 := 500, 450

	analyzer := &Analyzer{
		items: []ItemData{
			{
				ItemID:         1,
				Name:           "Item 1",
				InstaBuyPrice:  &buyPrice1,
				InstaSellPrice: &sellPrice1,
				MarginGP:       150, // Good margin
				BuyLimit:       100,
				FlipEfficiency: 15.0, // High efficiency
			},
			{
				ItemID:         2,
				Name:           "Item 2",
				InstaBuyPrice:  &buyPrice2,
				InstaSellPrice: &sellPrice2,
				MarginGP:       200, // Better margin
				BuyLimit:       50,
				FlipEfficiency: 20.0, // Higher efficiency
			},
			{
				ItemID:         3,
				Name:           "Item 3",
				InstaBuyPrice:  &buyPrice3,
				InstaSellPrice: &sellPrice3,
				MarginGP:       50, // Low margin, should be filtered out
				BuyLimit:       200,
				FlipEfficiency: 5.0,
			},
			{
				ItemID:         4,
				Name:           "Item 4",
				InstaBuyPrice:  nil, // Missing price data, should be filtered out
				InstaSellPrice: &sellPrice1,
				MarginGP:       150,
				BuyLimit:       100,
				FlipEfficiency: 10.0,
			},
		},
	}

	t.Run("get top items", func(t *testing.T) {
		itemIDs := analyzer.getTopItemIDs(10)

		// Should return items sorted by flip efficiency, filtering out invalid ones
		expectedIDs := []int{2, 1} // Item 2 has higher efficiency than Item 1
		if len(itemIDs) != len(expectedIDs) {
			t.Errorf("Expected %d items, got %d", len(expectedIDs), len(itemIDs))
		}

		for i, expectedID := range expectedIDs {
			if i >= len(itemIDs) || itemIDs[i] != expectedID {
				t.Errorf("Expected item ID %d at position %d, got %d", expectedID, i, itemIDs[i])
			}
		}
	})

	t.Run("limit results", func(t *testing.T) {
		itemIDs := analyzer.getTopItemIDs(1)

		// Should return only 1 item
		if len(itemIDs) != 1 {
			t.Errorf("Expected 1 item, got %d", len(itemIDs))
		}

		// Should be the highest efficiency item (Item 2)
		if itemIDs[0] != 2 {
			t.Errorf("Expected item ID 2, got %d", itemIDs[0])
		}
	})

	t.Run("empty analyzer", func(t *testing.T) {
		emptyAnalyzer := &Analyzer{items: []ItemData{}}
		itemIDs := emptyAnalyzer.getTopItemIDs(10)

		if len(itemIDs) != 0 {
			t.Errorf("Expected 0 items, got %d", len(itemIDs))
		}
	})
}

func TestUpdateItemsWithVolumeData(t *testing.T) {
	// Create mock analyzer with items
	buyPrice1, sellPrice1 := 1000, 950

	analyzer := &Analyzer{
		items: []ItemData{
			{
				ItemID:         1,
				Name:           "Test Item",
				InstaBuyPrice:  &buyPrice1,
				InstaSellPrice: &sellPrice1,
				MarginGP:       50,
				BuyLimit:       100,
			},
			{
				ItemID:         2,
				Name:           "Another Item",
				InstaBuyPrice:  &buyPrice1,
				InstaSellPrice: &sellPrice1,
				MarginGP:       75,
				BuyLimit:       50,
			},
		},
	}

	// Create mock volume data
	volumeData := map[int]VolumeMetrics{
		1: {
			InstaBuyVolume20m:      100.0,
			InstaSellVolume20m:     150.0,
			AvgInstaBuyPrice20m:    1005.0,
			AvgInstaSellPrice20m:   955.0,
			AvgMarginGP20m:         50.0,
			InstaBuyVolume1h:       500.0,
			InstaSellVolume1h:      600.0,
			AvgInstaBuyPrice1h:     1010.0,
			AvgInstaSellPrice1h:    960.0,
			AvgMarginGP1h:          50.0,
			InstaBuyPriceTrend1h:   "increasing",
			InstaSellPriceTrend1h:  "flat",
			InstaBuyPriceTrend24h:  "decreasing",
			InstaSellPriceTrend24h: "increasing",
			InstaBuyPriceTrend1w:   "flat",
			InstaSellPriceTrend1w:  "decreasing",
			InstaBuyPriceTrend1m:   "increasing",
			InstaSellPriceTrend1m:  "flat",
		},
	}

	analyzer.updateItemsWithVolumeData(volumeData)

	t.Run("volume data updated", func(t *testing.T) {
		item := &analyzer.items[0] // Item with ID 1

		// Check that volume metrics were updated
		if item.InstaBuyVolume20m == nil || *item.InstaBuyVolume20m != 100.0 {
			t.Errorf("Expected InstaBuyVolume20m to be 100.0, got %v", item.InstaBuyVolume20m)
		}

		if item.AvgInstaBuyPrice1h == nil || *item.AvgInstaBuyPrice1h != 1010.0 {
			t.Errorf("Expected AvgInstaBuyPrice1h to be 1010.0, got %v", item.AvgInstaBuyPrice1h)
		}

		// Check that trend data was updated
		if item.InstaBuyPriceTrend1h == nil || *item.InstaBuyPriceTrend1h != "increasing" {
			t.Errorf("Expected InstaBuyPriceTrend1h to be 'increasing', got %v", item.InstaBuyPriceTrend1h)
		}

		if item.InstaSellPriceTrend24h == nil || *item.InstaSellPriceTrend24h != "increasing" {
			t.Errorf("Expected InstaSellPriceTrend24h to be 'increasing', got %v", item.InstaSellPriceTrend24h)
		}
	})

	t.Run("item without volume data unchanged", func(t *testing.T) {
		item := &analyzer.items[1] // Item with ID 2 (no volume data)

		// Should remain nil since no volume data was provided
		if item.InstaBuyVolume20m != nil {
			t.Errorf("Expected InstaBuyVolume20m to remain nil, got %v", item.InstaBuyVolume20m)
		}

		if item.InstaBuyPriceTrend1h != nil {
			t.Errorf("Expected InstaBuyPriceTrend1h to remain nil, got %v", item.InstaBuyPriceTrend1h)
		}
	})
}

// Benchmark tests
func BenchmarkCalculateTrend(b *testing.B) {
	x := make([]float64, 100)
	y := make([]float64, 100)

	for i := 0; i < 100; i++ {
		x[i] = float64(i)
		y[i] = float64(i*2 + 1000) // Linear increasing trend
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculateTrend(x, y)
	}
}

func BenchmarkAverage(b *testing.B) {
	values := make([]float64, 1000)
	for i := 0; i < 1000; i++ {
		values[i] = float64(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		average(values)
	}
}
