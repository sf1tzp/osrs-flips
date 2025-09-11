package osrs

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestVolumeCalculationWith24hData(t *testing.T) {
	analyzer := &Analyzer{}

	// Create test data with current timestamps that fall within the 24h window
	now := time.Now().Unix()

	// Create 5-minute data points spanning the last 24 hours
	testData := map[string]interface{}{
		"data": []interface{}{
			// Data from 23 hours ago
			map[string]interface{}{
				"timestamp":       float64(now - (23 * 60 * 60)),
				"avgHighPrice":    1425999.0,
				"avgLowPrice":     1412664.0,
				"highPriceVolume": 2679.0,
				"lowPriceVolume":  3240.0,
			},
			// Data from 12 hours ago
			map[string]interface{}{
				"timestamp":       float64(now - (12 * 60 * 60)),
				"avgHighPrice":    1425194.0,
				"avgLowPrice":     1412955.0,
				"highPriceVolume": 2301.0,
				"lowPriceVolume":  2753.0,
			},
			// Data from 6 hours ago
			map[string]interface{}{
				"timestamp":       float64(now - (6 * 60 * 60)),
				"avgHighPrice":    1427616.0,
				"avgLowPrice":     1415302.0,
				"highPriceVolume": 2440.0,
				"lowPriceVolume":  2645.0,
			},
			// Data from 1 hour ago
			map[string]interface{}{
				"timestamp":       float64(now - (1 * 60 * 60)),
				"avgHighPrice":    1426110.0,
				"avgLowPrice":     1412850.0,
				"highPriceVolume": 2791.0,
				"lowPriceVolume":  3117.0,
			},
			// Recent data (30 minutes ago)
			map[string]interface{}{
				"timestamp":       float64(now - (30 * 60)),
				"avgHighPrice":    1422982.0,
				"avgLowPrice":     1407781.0,
				"highPriceVolume": 3298.0,
				"lowPriceVolume":  3486.0,
			},
		},
	}

	// Process the test data - pass as 5m data since that's where 24h volumes are calculated
	metrics := analyzer.processTimeseriesData(testData, map[string]interface{}{})

	t.Logf("Volume calculation results:")
	t.Logf("  InstaBuyVolume24h: %.2f", metrics.InstaBuyVolume24h)
	t.Logf("  InstaSellVolume24h: %.2f", metrics.InstaSellVolume24h)
	t.Logf("  AvgInstaBuyPrice24h: %.2f", metrics.AvgInstaBuyPrice24h)
	t.Logf("  AvgInstaSellPrice24h: %.2f", metrics.AvgInstaSellPrice24h)

	// Expected total volumes: 2679 + 2301 + 2440 + 2791 + 3298 = 13509 (buy volume)
	//                        3240 + 2753 + 2645 + 3117 + 3486 = 15241 (sell volume)
	expectedBuyVolume := 13509.0
	expectedSellVolume := 15241.0

	// Test that we actually calculated some volume
	if metrics.InstaBuyVolume24h == 0 {
		t.Error("Expected non-zero InstaBuyVolume24h, got 0")
	}

	if metrics.InstaSellVolume24h == 0 {
		t.Error("Expected non-zero InstaSellVolume24h, got 0")
	}

	// Test that volumes match expected totals
	if metrics.InstaBuyVolume24h != expectedBuyVolume {
		t.Errorf("InstaBuyVolume24h: expected %.2f, got %.2f", expectedBuyVolume, metrics.InstaBuyVolume24h)
	}

	if metrics.InstaSellVolume24h != expectedSellVolume {
		t.Errorf("InstaSellVolume24h: expected %.2f, got %.2f", expectedSellVolume, metrics.InstaSellVolume24h)
	}

	// Test that averages are reasonable
	expectedAvgBuyPrice := (1425999 + 1425194 + 1427616 + 1426110 + 1422982) / 5.0
	expectedAvgSellPrice := (1412664 + 1412955 + 1415302 + 1412850 + 1407781) / 5.0

	if metrics.AvgInstaBuyPrice24h != expectedAvgBuyPrice {
		t.Errorf("AvgInstaBuyPrice24h: expected %.2f, got %.2f", expectedAvgBuyPrice, metrics.AvgInstaBuyPrice24h)
	}

	if metrics.AvgInstaSellPrice24h != expectedAvgSellPrice {
		t.Errorf("AvgInstaSellPrice24h: expected %.2f, got %.2f", expectedAvgSellPrice, metrics.AvgInstaSellPrice24h)
	}
}

func TestVolumeCalculationDebugTimestamps(t *testing.T) {
	t.Skip("This test requires a current 'timeseries_24h_test_response.json to pass")

	// Read the actual 24h API response
	jsonData, err := os.ReadFile("timeseries_24h_test_response.json")
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	var apiResponse map[string]interface{}
	err = json.Unmarshal(jsonData, &apiResponse)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Check the timestamp ranges in the data
	if dataSlice, ok := apiResponse["data"].([]interface{}); ok {
		now := time.Now().Unix()
		window24h := now - (24 * 60 * 60)

		t.Logf("Current timestamp: %d (%s)", now, time.Unix(now, 0).UTC().Format("2006-01-02 15:04:05"))
		t.Logf("24h window starts at: %d (%s)", window24h, time.Unix(window24h, 0).UTC().Format("2006-01-02 15:04:05"))

		var validPoints, totalPoints int
		var firstTimestamp, lastTimestamp int64

		for i, item := range dataSlice {
			if dataPoint, ok := item.(map[string]interface{}); ok {
				if timestampVal, exists := dataPoint["timestamp"]; exists {
					timestamp := int64(timestampVal.(float64))

					if i == 0 {
						firstTimestamp = timestamp
					}
					if i == len(dataSlice)-1 {
						lastTimestamp = timestamp
					}

					totalPoints++
					if timestamp >= window24h {
						validPoints++
					}

					// Log first few and last few timestamps for debugging
					if i < 5 || i >= len(dataSlice)-5 {
						t.Logf("  Point %d: timestamp=%d (%s), within24h=%v",
							i, timestamp, time.Unix(timestamp, 0).UTC().Format("2006-01-02 15:04:05"),
							timestamp >= window24h)
					}
				}
			}
		}

		t.Logf("Data timestamp range:")
		t.Logf("  First: %d (%s)", firstTimestamp, time.Unix(firstTimestamp, 0).UTC().Format("2006-01-02 15:04:05"))
		t.Logf("  Last:  %d (%s)", lastTimestamp, time.Unix(lastTimestamp, 0).UTC().Format("2006-01-02 15:04:05"))
		t.Logf("Total points: %d, Points within 24h window: %d", totalPoints, validPoints)

		if validPoints == 0 {
			t.Errorf("No data points fall within the 24h window! All data appears to be older than 24h")
		}
	}
}
