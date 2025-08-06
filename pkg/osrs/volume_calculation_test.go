package osrs

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestVolumeCalculationWith24hData(t *testing.T) {
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

	analyzer := &Analyzer{}

	// Process the actual API data
	metrics := analyzer.processTimeseriesData(map[string]interface{}{}, apiResponse)

	t.Logf("Volume calculation results:")
	t.Logf("  InstaBuyVolume24h: %.2f", metrics.InstaBuyVolume24h)
	t.Logf("  InstaSellVolume24h: %.2f", metrics.InstaSellVolume24h)
	t.Logf("  AvgInstaBuyPrice24h: %.2f", metrics.AvgInstaBuyPrice24h)
	t.Logf("  AvgInstaSellPrice24h: %.2f", metrics.AvgInstaSellPrice24h)

	// Test that we actually calculated some volume
	if metrics.InstaBuyVolume24h == 0 {
		t.Error("Expected non-zero InstaBuyVolume24h, got 0")
	}

	if metrics.InstaSellVolume24h == 0 {
		t.Error("Expected non-zero InstaSellVolume24h, got 0")
	}

	// Test that volumes are reasonable (should be > 1 given your config expects volume_24h_min: 1)
	if metrics.InstaBuyVolume24h < 1 {
		t.Errorf("InstaBuyVolume24h (%.2f) is less than expected minimum of 1", metrics.InstaBuyVolume24h)
	}

	if metrics.InstaSellVolume24h < 1 {
		t.Errorf("InstaSellVolume24h (%.2f) is less than expected minimum of 1", metrics.InstaSellVolume24h)
	}

	// Based on your JSON data, we should see significant volume
	// The first few entries show volumes like 2679, 3240, 2301, 2753, etc.
	// Even with 24h filtering, we should get substantial totals
	expectedMinVolume := 1000.0 // Conservative estimate

	if metrics.InstaBuyVolume24h < expectedMinVolume {
		t.Errorf("InstaBuyVolume24h (%.2f) seems too low, expected at least %.2f",
			metrics.InstaBuyVolume24h, expectedMinVolume)
	}

	if metrics.InstaSellVolume24h < expectedMinVolume {
		t.Errorf("InstaSellVolume24h (%.2f) seems too low, expected at least %.2f",
			metrics.InstaSellVolume24h, expectedMinVolume)
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
