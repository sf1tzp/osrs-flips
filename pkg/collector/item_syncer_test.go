package collector

import (
	"testing"
	"time"
)

func TestDefaultItemSyncerConfig(t *testing.T) {
	config := DefaultItemSyncerConfig()

	if !config.SyncOnStart {
		t.Error("Expected SyncOnStart to be true by default")
	}

	if config.SyncInterval != 24*time.Hour {
		t.Errorf("Expected SyncInterval to be 24h, got %v", config.SyncInterval)
	}
}

func TestNewItemSyncer_NilConfig(t *testing.T) {
	// Should not panic with nil config
	syncer := NewItemSyncer(nil, nil, nil, nil)

	if syncer.config == nil {
		t.Error("Expected config to be set to defaults")
	}

	if !syncer.config.SyncOnStart {
		t.Error("Expected default SyncOnStart to be true")
	}
}
