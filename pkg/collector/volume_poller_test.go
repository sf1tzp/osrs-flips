package collector

import (
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestDefaultVolumePollerConfig(t *testing.T) {
	cfg := DefaultVolumePollerConfig()

	if cfg.PollInterval != 5*time.Minute {
		t.Errorf("PollInterval = %v, want 5m", cfg.PollInterval)
	}

	if cfg.RateLimit != 100*time.Millisecond {
		t.Errorf("RateLimit = %v, want 100ms", cfg.RateLimit)
	}

	if cfg.RetryDelay != 10*time.Second {
		t.Errorf("RetryDelay = %v, want 10s", cfg.RetryDelay)
	}

	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.MaxRetries)
	}

	if cfg.BackoffMax != 5*time.Minute {
		t.Errorf("BackoffMax = %v, want 5m", cfg.BackoffMax)
	}
}

func TestNewVolumePoller_NilConfig(t *testing.T) {
	vp := NewVolumePoller(nil, nil, nil, nil, nil)

	if vp.config == nil {
		t.Fatal("config should not be nil when passed nil")
	}

	if vp.config.PollInterval != 5*time.Minute {
		t.Errorf("PollInterval = %v, want 5m", vp.config.PollInterval)
	}

	if vp.limiter == nil {
		t.Error("limiter should be created when passed nil")
	}
}

func TestNewVolumePoller_ExternalLimiter(t *testing.T) {
	externalLimiter := rate.NewLimiter(rate.Every(time.Millisecond), 1)

	vp := NewVolumePoller(nil, nil, nil, nil, externalLimiter)

	if vp.limiter != externalLimiter {
		t.Error("should use external limiter when provided")
	}
}

func TestVolumePollerProgress_Initial(t *testing.T) {
	vp := NewVolumePoller(nil, nil, nil, nil, nil)
	progress := vp.Progress()

	if progress.CyclesCompleted != 0 {
		t.Errorf("CyclesCompleted = %d, want 0", progress.CyclesCompleted)
	}
	if progress.ItemsPolled != 0 {
		t.Errorf("ItemsPolled = %d, want 0", progress.ItemsPolled)
	}
	if progress.BucketsFilled != 0 {
		t.Errorf("BucketsFilled = %d, want 0", progress.BucketsFilled)
	}
	if progress.Errors != 0 {
		t.Errorf("Errors = %d, want 0", progress.Errors)
	}
}

func TestVolumePoller_Running(t *testing.T) {
	vp := NewVolumePoller(nil, nil, nil, nil, nil)

	if vp.Running() {
		t.Error("should not be running initially")
	}
}

func TestVolumePoller_Stats(t *testing.T) {
	vp := NewVolumePoller(nil, nil, nil, nil, nil)
	stats := vp.Stats()

	if stats["running"] != false {
		t.Errorf("running = %v, want false", stats["running"])
	}
	if stats["consecutive_fails"] != 0 {
		t.Errorf("consecutive_fails = %v, want 0", stats["consecutive_fails"])
	}
	if stats["cycles_completed"] != 0 {
		t.Errorf("cycles_completed = %v, want 0", stats["cycles_completed"])
	}
	if stats["poll_interval"] != "5m0s" {
		t.Errorf("poll_interval = %v, want 5m0s", stats["poll_interval"])
	}
}

func TestVolumePoller_StartStop_NoOp(t *testing.T) {
	// Test that Start/Stop don't panic with nil dependencies
	// Use a long interval so it doesn't try to poll during test
	vp := NewVolumePoller(nil, nil, &VolumePollerConfig{
		PollInterval: time.Hour,
		RateLimit:    time.Millisecond,
		RetryDelay:   time.Second,
		MaxRetries:   1,
		BackoffMax:   time.Second,
	}, nil, nil)

	// Double start should be no-op
	vp.Start()
	vp.Start() // Should not panic or block

	if !vp.Running() {
		t.Error("should be running after Start")
	}

	// Stop should work
	vp.Stop()

	if vp.Running() {
		t.Error("should not be running after Stop")
	}

	// Double stop should be no-op
	vp.Stop() // Should not panic or block
}
