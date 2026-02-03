package collector

import (
	"testing"
	"time"
)

func TestRetentionPolicy(t *testing.T) {
	tests := []struct {
		bucketSize string
		want       time.Duration
	}{
		{"5m", 7 * 24 * time.Hour},
		{"1h", 365 * 24 * time.Hour},
		{"24h", 0},
	}

	for _, tt := range tests {
		t.Run(tt.bucketSize, func(t *testing.T) {
			got := RetentionPolicy[tt.bucketSize]
			if got != tt.want {
				t.Errorf("RetentionPolicy[%q] = %v, want %v", tt.bucketSize, got, tt.want)
			}
		})
	}
}

func TestDefaultGapFillerConfig(t *testing.T) {
	cfg := DefaultGapFillerConfig()

	if cfg.ItemsPerRun != 150 {
		t.Errorf("ItemsPerRun = %d, want 150", cfg.ItemsPerRun)
	}

	if cfg.RateLimit != 100*time.Millisecond {
		t.Errorf("RateLimit = %v, want 100ms", cfg.RateLimit)
	}

	if cfg.MaxConcurrent != 1 {
		t.Errorf("MaxConcurrent = %d, want 1", cfg.MaxConcurrent)
	}

	expectedBuckets := []string{"5m", "1h", "24h"}
	if len(cfg.BucketSizes) != len(expectedBuckets) {
		t.Errorf("BucketSizes length = %d, want %d", len(cfg.BucketSizes), len(expectedBuckets))
	}
	for i, bucket := range expectedBuckets {
		if cfg.BucketSizes[i] != bucket {
			t.Errorf("BucketSizes[%d] = %q, want %q", i, cfg.BucketSizes[i], bucket)
		}
	}
}

func TestNewGapFiller_NilConfig(t *testing.T) {
	// Should use defaults when config is nil
	gf := NewGapFiller(nil, nil, nil, nil)

	if gf.config == nil {
		t.Fatal("config should not be nil when passed nil")
	}

	if gf.config.ItemsPerRun != 150 {
		t.Errorf("ItemsPerRun = %d, want 150", gf.config.ItemsPerRun)
	}
}

func TestGapFillerProgress_Initial(t *testing.T) {
	gf := NewGapFiller(nil, nil, nil, nil)
	progress := gf.Progress()

	if progress.ItemsScanned != 0 {
		t.Errorf("ItemsScanned = %d, want 0", progress.ItemsScanned)
	}
	if progress.GapsFound != 0 {
		t.Errorf("GapsFound = %d, want 0", progress.GapsFound)
	}
	if progress.BucketsFilled != 0 {
		t.Errorf("BucketsFilled = %d, want 0", progress.BucketsFilled)
	}
}
