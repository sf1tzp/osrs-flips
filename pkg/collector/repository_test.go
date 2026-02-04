package collector

import "testing"

func TestBucketTableName(t *testing.T) {
	tests := []struct {
		bucketSize string
		want       string
	}{
		{"5m", "price_buckets_5m"},
		{"1h", "price_buckets_1h"},
		{"24h", "price_buckets_24h"},
		{"invalid", "price_buckets_5m"}, // fallback
		{"", "price_buckets_5m"},        // fallback
	}

	for _, tt := range tests {
		t.Run(tt.bucketSize, func(t *testing.T) {
			got := bucketTableName(tt.bucketSize)
			if got != tt.want {
				t.Errorf("bucketTableName(%q) = %q, want %q", tt.bucketSize, got, tt.want)
			}
		})
	}
}

// Note: GetItemsNeedingSync requires database integration tests.
// The SQL query logic is tested implicitly through the application.
// Consider adding testcontainers-based integration tests in the future.
