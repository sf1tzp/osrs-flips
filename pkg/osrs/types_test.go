package osrs

import (
	"testing"
	"time"
)

func TestVolumeMetrics(t *testing.T) {
	t.Run("VolumeMetrics struct initialization", func(t *testing.T) {
		metrics := VolumeMetrics{
			InstaBuyVolume20m:      100.0,
			InstaSellVolume20m:     150.0,
			AvgInstaBuyPrice20m:    1000.0,
			AvgInstaSellPrice20m:   950.0,
			AvgMarginGP20m:         50.0,
			InstaBuyPriceTrend1h:   "increasing",
			InstaSellPriceTrend1h:  "decreasing",
			InstaBuyPriceTrend24h:  "flat",
			InstaSellPriceTrend24h: "increasing",
			InstaBuyPriceTrend1w:   "decreasing",
			InstaSellPriceTrend1w:  "flat",
			InstaBuyPriceTrend1m:   "increasing",
			InstaSellPriceTrend1m:  "flat",
		}

		if metrics.InstaBuyVolume20m != 100.0 {
			t.Errorf("Expected InstaBuyVolume20m to be 100.0, got %f", metrics.InstaBuyVolume20m)
		}

		if metrics.InstaBuyPriceTrend1h != "increasing" {
			t.Errorf("Expected InstaBuyPriceTrend1h to be 'increasing', got %s", metrics.InstaBuyPriceTrend1h)
		}

		if metrics.AvgMarginGP20m != 50.0 {
			t.Errorf("Expected AvgMarginGP20m to be 50.0, got %f", metrics.AvgMarginGP20m)
		}
	})
}

func TestVolumeDataPoint(t *testing.T) {
	t.Run("VolumeDataPoint struct with nil values", func(t *testing.T) {
		dataPoint := VolumeDataPoint{
			Timestamp:    1234567890,
			AvgHighPrice: nil,
			AvgLowPrice:  nil,
			HighPriceVol: nil,
			LowPriceVol:  nil,
		}

		if dataPoint.Timestamp != 1234567890 {
			t.Errorf("Expected Timestamp to be 1234567890, got %d", dataPoint.Timestamp)
		}

		if dataPoint.AvgHighPrice != nil {
			t.Errorf("Expected AvgHighPrice to be nil, got %v", dataPoint.AvgHighPrice)
		}
	})

	t.Run("VolumeDataPoint struct with values", func(t *testing.T) {
		highPrice := 1000
		lowPrice := 950
		highVol := 100
		lowVol := 150

		dataPoint := VolumeDataPoint{
			Timestamp:    1234567890,
			AvgHighPrice: &highPrice,
			AvgLowPrice:  &lowPrice,
			HighPriceVol: &highVol,
			LowPriceVol:  &lowVol,
		}

		if *dataPoint.AvgHighPrice != 1000 {
			t.Errorf("Expected AvgHighPrice to be 1000, got %d", *dataPoint.AvgHighPrice)
		}

		if *dataPoint.LowPriceVol != 150 {
			t.Errorf("Expected LowPriceVol to be 150, got %d", *dataPoint.LowPriceVol)
		}
	})
}

func TestTimeseriesResponse(t *testing.T) {
	t.Run("TimeseriesResponse struct", func(t *testing.T) {
		highPrice := 1000
		lowPrice := 950

		response := TimeseriesResponse{
			Data: []VolumeDataPoint{
				{
					Timestamp:    1234567890,
					AvgHighPrice: &highPrice,
					AvgLowPrice:  &lowPrice,
				},
				{
					Timestamp:    1234567900,
					AvgHighPrice: nil,
					AvgLowPrice:  nil,
				},
			},
		}

		if len(response.Data) != 2 {
			t.Errorf("Expected Data to have 2 elements, got %d", len(response.Data))
		}

		if *response.Data[0].AvgHighPrice != 1000 {
			t.Errorf("Expected first data point AvgHighPrice to be 1000, got %d", *response.Data[0].AvgHighPrice)
		}

		if response.Data[1].AvgHighPrice != nil {
			t.Errorf("Expected second data point AvgHighPrice to be nil, got %v", response.Data[1].AvgHighPrice)
		}
	})
}

func TestCalculate5mMetricsEdgeCases(t *testing.T) {
	analyzer := &Analyzer{}

	t.Run("no data points", func(t *testing.T) {
		dataSlice := []interface{}{}
		metrics := VolumeMetrics{}

		result := analyzer.calculate5mMetrics(dataSlice, metrics)

		if result.InstaBuyVolume20m != 0 {
			t.Errorf("Expected InstaBuyVolume20m to be 0, got %f", result.InstaBuyVolume20m)
		}

		if result.InstaBuyPriceTrend1h != "flat" {
			t.Errorf("Expected InstaBuyPriceTrend1h to be 'flat', got %s", result.InstaBuyPriceTrend1h)
		}
	})

	t.Run("data points with nil prices", func(t *testing.T) {
		// Use current time minus a few minutes to ensure it's within the 20m window
		now := time.Now().Unix()
		recentTimestamp := now - 300 // 5 minutes ago

		dataSlice := []interface{}{
			map[string]interface{}{
				"timestamp":       float64(recentTimestamp),
				"avgHighPrice":    nil,
				"avgLowPrice":     nil,
				"highPriceVolume": 100.0,
				"lowPriceVolume":  150.0,
			},
		}
		metrics := VolumeMetrics{}

		result := analyzer.calculate5mMetrics(dataSlice, metrics)

		if result.InstaBuyVolume20m != 100.0 {
			t.Errorf("Expected InstaBuyVolume20m to be 100.0, got %f", result.InstaBuyVolume20m)
		}

		if result.AvgInstaBuyPrice20m != 0 {
			t.Errorf("Expected AvgInstaBuyPrice20m to be 0, got %f", result.AvgInstaBuyPrice20m)
		}
	})

	t.Run("data points with zero prices", func(t *testing.T) {
		// Use current time minus a few minutes to ensure it's within the 20m window
		now := time.Now().Unix()
		recentTimestamp := now - 300 // 5 minutes ago

		dataSlice := []interface{}{
			map[string]interface{}{
				"timestamp":       float64(recentTimestamp),
				"avgHighPrice":    0.0,
				"avgLowPrice":     0.0,
				"highPriceVolume": 100.0,
				"lowPriceVolume":  150.0,
			},
		}
		metrics := VolumeMetrics{}

		result := analyzer.calculate5mMetrics(dataSlice, metrics)

		// Zero prices should not be included in price arrays
		if result.AvgInstaBuyPrice20m != 0 {
			t.Errorf("Expected AvgInstaBuyPrice20m to be 0, got %f", result.AvgInstaBuyPrice20m)
		}

		// But volume should still be accumulated
		if result.InstaBuyVolume20m != 100.0 {
			t.Errorf("Expected InstaBuyVolume20m to be 100.0, got %f", result.InstaBuyVolume20m)
		}
	})
}

func TestCalculate24hMetricsEdgeCases(t *testing.T) {
	analyzer := &Analyzer{}

	t.Run("insufficient data for trends", func(t *testing.T) {
		// Only 1 data point - insufficient for trend calculation (needs 3+)
		dataSlice := []interface{}{
			map[string]interface{}{
				"timestamp":       float64(1234567890),
				"avgHighPrice":    1000.0,
				"avgLowPrice":     950.0,
				"highPriceVolume": 100.0,
				"lowPriceVolume":  150.0,
			},
		}
		metrics := VolumeMetrics{}

		result := analyzer.calculate24hMetrics(dataSlice, metrics)

		if result.InstaBuyPriceTrend24h != "flat" {
			t.Errorf("Expected InstaBuyPriceTrend24h to be 'flat', got %s", result.InstaBuyPriceTrend24h)
		}

		if result.InstaBuyPriceTrend1w != "flat" {
			t.Errorf("Expected InstaBuyPriceTrend1w to be 'flat', got %s", result.InstaBuyPriceTrend1w)
		}

		if result.InstaBuyPriceTrend1m != "flat" {
			t.Errorf("Expected InstaBuyPriceTrend1m to be 'flat', got %s", result.InstaBuyPriceTrend1m)
		}
	})

	t.Run("invalid data structure", func(t *testing.T) {
		// Data that can't be converted to map[string]interface{}
		dataSlice := []interface{}{
			"invalid_data",
			12345,
		}
		metrics := VolumeMetrics{}

		result := analyzer.calculate24hMetrics(dataSlice, metrics)

		// Should handle gracefully and return empty metrics
		if result.InstaBuyVolume24h != 0 {
			t.Errorf("Expected InstaBuyVolume24h to be 0, got %f", result.InstaBuyVolume24h)
		}

		if result.InstaBuyPriceTrend24h != "flat" {
			t.Errorf("Expected InstaBuyPriceTrend24h to be 'flat', got %s", result.InstaBuyPriceTrend24h)
		}
	})
}

func TestProcessTimeseriesDataEdgeCases(t *testing.T) {
	analyzer := &Analyzer{}

	t.Run("invalid data structure - not array", func(t *testing.T) {
		data5m := map[string]interface{}{
			"data": "invalid_array",
		}
		data24h := map[string]interface{}{
			"data": 12345,
		}

		metrics := analyzer.processTimeseriesData(data5m, data24h)

		// Should handle gracefully
		if metrics.InstaBuyVolume20m != 0 {
			t.Errorf("Expected InstaBuyVolume20m to be 0, got %f", metrics.InstaBuyVolume20m)
		}
	})

	t.Run("missing data key", func(t *testing.T) {
		data5m := map[string]interface{}{
			"other_key": []interface{}{},
		}
		data24h := map[string]interface{}{}

		metrics := analyzer.processTimeseriesData(data5m, data24h)

		// Should handle gracefully
		if metrics.InstaBuyVolume20m != 0 {
			t.Errorf("Expected InstaBuyVolume20m to be 0, got %f", metrics.InstaBuyVolume20m)
		}
	})
}

// Test edge cases for the trending calculation with real-world scenarios
func TestCalculateTrendRealWorldScenarios(t *testing.T) {
	tests := []struct {
		name        string
		x           []float64
		y           []float64
		expected    string
		description string
	}{
		{
			name:        "market crash scenario",
			x:           []float64{1, 2, 3, 4, 5, 6, 7},
			y:           []float64{10000, 9500, 8000, 7000, 6000, 5500, 5000}, // 50% crash
			expected:    "decreasing",
			description: "Should detect severe market decline",
		},
		{
			name:        "market bubble scenario",
			x:           []float64{1, 2, 3, 4, 5, 6},
			y:           []float64{1000, 1200, 1500, 2000, 2800, 3500}, // 250% increase
			expected:    "increasing",
			description: "Should detect rapid price increase",
		},
		{
			name:        "sideways trading with noise",
			x:           []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			y:           []float64{1000, 1005, 995, 1008, 992, 1003, 997, 1006, 994, 1001}, // 0.1% overall
			expected:    "flat",
			description: "Should detect sideways movement despite volatility",
		},
		{
			name:        "gradual uptrend",
			x:           []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			y:           []float64{1000, 1003, 1007, 1012, 1016, 1021, 1025, 1030, 1034, 1040}, // 4% gradual increase
			expected:    "increasing",
			description: "Should detect gradual but consistent uptrend",
		},
		{
			name:        "very large numbers",
			x:           []float64{1640995200, 1640995260, 1640995320, 1640995380, 1640995440}, // Unix timestamps
			y:           []float64{2147483647, 2147483700, 2147483800, 2147483900, 2200000000}, // Large integers
			expected:    "increasing",
			description: "Should handle large numbers correctly",
		},
		{
			name:        "very small price movements",
			x:           []float64{1, 2, 3, 4, 5},
			y:           []float64{0.001, 0.002, 0.003, 0.004, 0.005}, // 400% but tiny absolute values
			expected:    "increasing",
			description: "Should handle very small absolute values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateTrend(tt.x, tt.y)
			if result != tt.expected {
				t.Errorf("%s: calculateTrend() = %v, want %v", tt.description, result, tt.expected)

				// Print additional debug info for failed tests
				if len(tt.y) > 1 && tt.y[0] != 0 {
					pctChange := (tt.y[len(tt.y)-1] - tt.y[0]) / tt.y[0] * 100
					t.Logf("Percentage change: %.2f%%", pctChange)
				}
			}
		})
	}
}
