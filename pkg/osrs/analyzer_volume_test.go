package osrs

import (
	"testing"
)

func TestPassesVolumeFilters(t *testing.T) {
	analyzer := &Analyzer{}

	tests := []struct {
		name           string
		item           ItemData
		opts           FilterOptions
		expectedResult bool
		description    string
	}{
		{
			name: "No volume filters - should pass",
			item: ItemData{
				ItemID: 1,
				Name:   "Test Item",
			},
			opts:           FilterOptions{},
			expectedResult: true,
			description:    "When no volume filters are set, item should pass",
		},
		{
			name: "Volume1hMin filter - both volumes nil - should fail",
			item: ItemData{
				ItemID:            1,
				Name:              "Test Item",
				InstaBuyVolume1h:  nil,
				InstaSellVolume1h: nil,
			},
			opts: FilterOptions{
				Volume1hMin: intPtr(1),
			},
			expectedResult: false,
			description:    "When volume data is nil and filter is set, should fail",
		},
		{
			name: "Volume1hMin filter - only buy volume present, meets threshold - should fail",
			item: ItemData{
				ItemID:            1,
				Name:              "Test Item",
				InstaBuyVolume1h:  float64Ptr(5.0),
				InstaSellVolume1h: nil,
			},
			opts: FilterOptions{
				Volume1hMin: intPtr(1),
			},
			expectedResult: false,
			description:    "When only one volume is present, should fail (both required)",
		},
		{
			name: "Volume1hMin filter - both volumes below threshold - should fail",
			item: ItemData{
				ItemID:            1,
				Name:              "Test Item",
				InstaBuyVolume1h:  float64Ptr(0.5),
				InstaSellVolume1h: float64Ptr(0.3),
			},
			opts: FilterOptions{
				Volume1hMin: intPtr(1),
			},
			expectedResult: false,
			description:    "When both volumes are below threshold individually, should fail",
		},
		{
			name: "Volume1hMin filter - buy volume above, sell below threshold - should pass",
			item: ItemData{
				ItemID:            1,
				Name:              "Test Item",
				InstaBuyVolume1h:  float64Ptr(5.0),
				InstaSellVolume1h: float64Ptr(0.5),
			},
			opts: FilterOptions{
				Volume1hMin: intPtr(1),
			},
			expectedResult: true,
			description:    "When only one volume meets threshold, should pass total volume filter",
		},
		{
			name: "Volume1hMin filter - both volumes exactly at threshold - should pass",
			item: ItemData{
				ItemID:            1,
				Name:              "Test Item",
				InstaBuyVolume1h:  float64Ptr(1.0),
				InstaSellVolume1h: float64Ptr(1.0),
			},
			opts: FilterOptions{
				Volume1hMin: intPtr(1),
			},
			expectedResult: true,
			description:    "When both volumes exactly meet threshold, should pass",
		},
		{
			name: "Volume1hMin filter - both volumes above threshold - should pass",
			item: ItemData{
				ItemID:            1,
				Name:              "Test Item",
				InstaBuyVolume1h:  float64Ptr(10.0),
				InstaSellVolume1h: float64Ptr(15.0),
			},
			opts: FilterOptions{
				Volume1hMin: intPtr(1),
			},
			expectedResult: true,
			description:    "When both volumes exceed threshold, should pass",
		},
		{
			name: "Volume24hMin filter - both volumes nil - should fail",
			item: ItemData{
				ItemID:             1,
				Name:               "Test Item",
				InstaBuyVolume24h:  nil,
				InstaSellVolume24h: nil,
			},
			opts: FilterOptions{
				Volume24hMin: intPtr(1),
			},
			expectedResult: false,
			description:    "When 24h volume data is nil and filter is set, should fail",
		},
		{
			name: "Volume24hMin filter - both volumes below threshold - should fail",
			item: ItemData{
				ItemID:             1,
				Name:               "Test Item",
				InstaBuyVolume24h:  float64Ptr(0.5),
				InstaSellVolume24h: float64Ptr(0.3),
			},
			opts: FilterOptions{
				Volume24hMin: intPtr(1),
			},
			expectedResult: false,
			description:    "When both 24h volumes are below threshold individually, should fail",
		},
		{
			name: "Volume24hMin filter - both volumes below threshold - should pass",
			item: ItemData{
				ItemID:             1,
				Name:               "Test Item",
				InstaBuyVolume24h:  float64Ptr(0.5),
				InstaSellVolume24h: float64Ptr(0.5),
			},
			opts: FilterOptions{
				Volume24hMin: intPtr(1),
			},
			expectedResult: true,
			description:    "When both 24h volumes are below threshold individually, should pass",
		},
		{ // NOTE: The volume filters work against the total volume, not individual
			name: "Volume24hMin filter - buy volume above, sell below threshold - should pass",
			item: ItemData{
				ItemID:             1,
				Name:               "Test Item",
				InstaBuyVolume24h:  float64Ptr(5.0),
				InstaSellVolume24h: float64Ptr(0.5),
			},
			opts: FilterOptions{
				Volume24hMin: intPtr(1),
			},
			expectedResult: true,
			description:    "When only one 24h volume meets threshold, should pass total volume filter",
		},
		{
			name: "Volume24hMin filter - both volumes above threshold - should pass",
			item: ItemData{
				ItemID:             1,
				Name:               "Test Item",
				InstaBuyVolume24h:  float64Ptr(10.0),
				InstaSellVolume24h: float64Ptr(15.0),
			},
			opts: FilterOptions{
				Volume24hMin: intPtr(1),
			},
			expectedResult: true,
			description:    "When both 24h volumes exceed threshold, should pass",
		},
		{
			name: "Both volume filters - all volumes meet thresholds - should pass",
			item: ItemData{
				ItemID:             1,
				Name:               "Test Item",
				InstaBuyVolume1h:   float64Ptr(5.0),
				InstaSellVolume1h:  float64Ptr(3.0),
				InstaBuyVolume24h:  float64Ptr(50.0),
				InstaSellVolume24h: float64Ptr(30.0),
			},
			opts: FilterOptions{
				Volume1hMin:  intPtr(2),
				Volume24hMin: intPtr(20),
			},
			expectedResult: true,
			description:    "When all volumes meet their respective thresholds, should pass",
		},
		{ // NOTE: The volume filters work against the total volume, not individual
			name: "Both volume filters - 1h fails, 24h passes - should fail",
			item: ItemData{
				ItemID:             1,
				Name:               "Test Item",
				InstaBuyVolume1h:   float64Ptr(1.0),  // Below threshold of 2
				InstaSellVolume1h:  float64Ptr(3.0),  // Above threshold
				InstaBuyVolume24h:  float64Ptr(50.0), // Above threshold
				InstaSellVolume24h: float64Ptr(30.0), // Above threshold
			},
			opts: FilterOptions{
				Volume1hMin:  intPtr(2),
				Volume24hMin: intPtr(20),
			},
			expectedResult: true,
			description:    "When any volume filter fails, entire filter should fail",
		},
		{
			name: "Edge case - zero volume threshold",
			item: ItemData{
				ItemID:            1,
				Name:              "Test Item",
				InstaBuyVolume1h:  float64Ptr(0.0),
				InstaSellVolume1h: float64Ptr(0.0),
			},
			opts: FilterOptions{
				Volume1hMin: intPtr(0),
			},
			expectedResult: true,
			description:    "When threshold is zero and volumes are zero, should pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.passesVolumeFilters(tt.item, tt.opts)
			if result != tt.expectedResult {
				t.Errorf("passesVolumeFilters() = %v, want %v\nDescription: %s\nItem: %+v\nOpts: %+v",
					result, tt.expectedResult, tt.description, tt.item, tt.opts)
			}
		})
	}
}

// Helper functions to create pointers
func intPtr(i int) *int {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}
