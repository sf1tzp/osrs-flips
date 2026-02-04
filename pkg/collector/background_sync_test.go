package collector

import (
	"testing"
	"time"

	"golang.org/x/time/rate"
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

func TestDefaultBackgroundSyncConfig(t *testing.T) {
	cfg := DefaultBackgroundSyncConfig()

	if cfg.RunInterval != 5*time.Minute {
		t.Errorf("RunInterval = %v, want 5m", cfg.RunInterval)
	}

	if cfg.TimestampsPerCycle != 50 {
		t.Errorf("TimestampsPerCycle = %d, want 50", cfg.TimestampsPerCycle)
	}

	if cfg.MinItemThreshold != 100 {
		t.Errorf("MinItemThreshold = %d, want 100", cfg.MinItemThreshold)
	}

	if cfg.RateLimit != 100*time.Millisecond {
		t.Errorf("RateLimit = %v, want 100ms", cfg.RateLimit)
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

func TestNewBackgroundSync_NilConfig(t *testing.T) {
	bs := NewBackgroundSync(nil, nil, nil, nil, nil)

	if bs.config == nil {
		t.Fatal("config should not be nil when passed nil")
	}

	if bs.config.RunInterval != 5*time.Minute {
		t.Errorf("RunInterval = %v, want 5m", bs.config.RunInterval)
	}

	if bs.limiter == nil {
		t.Error("limiter should be created when passed nil")
	}
}

func TestNewBackgroundSync_ExternalLimiter(t *testing.T) {
	// Create an external limiter with different rate
	externalLimiter := rate.NewLimiter(rate.Every(time.Millisecond), 1)

	bs := NewBackgroundSync(nil, nil, nil, nil, externalLimiter)

	if bs.limiter != externalLimiter {
		t.Error("should use external limiter when provided")
	}
}

func TestBackgroundSyncProgress_Initial(t *testing.T) {
	bs := NewBackgroundSync(nil, nil, nil, nil, nil)
	progress := bs.Progress()

	if progress.CyclesCompleted != 0 {
		t.Errorf("CyclesCompleted = %d, want 0", progress.CyclesCompleted)
	}
	if progress.TimestampsSynced != 0 {
		t.Errorf("TimestampsSynced = %d, want 0", progress.TimestampsSynced)
	}
	if progress.BucketsFilled != 0 {
		t.Errorf("BucketsFilled = %d, want 0", progress.BucketsFilled)
	}
	if progress.Errors != 0 {
		t.Errorf("Errors = %d, want 0", progress.Errors)
	}
}

func TestBackgroundSync_Running(t *testing.T) {
	bs := NewBackgroundSync(nil, nil, nil, nil, nil)

	if bs.Running() {
		t.Error("should not be running initially")
	}
}

func TestBackgroundSync_StartStop_NoOp(t *testing.T) {
	// Test that Start/Stop don't panic with nil dependencies
	// (they will fail during actual sync, but the lifecycle should work)
	bs := NewBackgroundSync(nil, nil, &BackgroundSyncConfig{
		BucketSizes:       []string{},
		RunInterval:       time.Hour, // Long interval so it doesn't try to run
		TimestampsPerCycle: 0,
		MinItemThreshold:  100,
		RateLimit:         time.Millisecond,
	}, nil, nil)

	// Double start should be no-op
	bs.Start()
	bs.Start() // Should not panic or block

	if !bs.Running() {
		t.Error("should be running after Start")
	}

	// Stop should work
	bs.Stop()

	if bs.Running() {
		t.Error("should not be running after Stop")
	}

	// Double stop should be no-op
	bs.Stop() // Should not panic or block
}
