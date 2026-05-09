package collector

import (
	"context"
	"time"

	"osrs-flipping/pkg/logging"
	"osrs-flipping/pkg/osrs"
)

// ItemSyncerConfig configures the item syncer service.
type ItemSyncerConfig struct {
	// SyncOnStart triggers a sync when the service starts
	SyncOnStart bool
	// SyncInterval is the interval between automatic syncs (0 = no auto-sync)
	SyncInterval time.Duration
}

// DefaultItemSyncerConfig returns sensible defaults.
func DefaultItemSyncerConfig() *ItemSyncerConfig {
	return &ItemSyncerConfig{
		SyncOnStart:  true,
		SyncInterval: 24 * time.Hour, // Daily refresh
	}
}

// ItemSyncer populates and refreshes the items table from the OSRS Wiki API.
type ItemSyncer struct {
	client *osrs.Client
	repo   *Repository
	config *ItemSyncerConfig
	logger *logging.Logger
}

// NewItemSyncer creates a new ItemSyncer.
func NewItemSyncer(client *osrs.Client, repo *Repository, config *ItemSyncerConfig, logger *logging.Logger) *ItemSyncer {
	if config == nil {
		config = DefaultItemSyncerConfig()
	}
	return &ItemSyncer{
		client: client,
		repo:   repo,
		config: config,
		logger: logger,
	}
}

// Sync fetches item mappings from the API and upserts them into the database.
// This is idempotent and safe to call multiple times.
func (s *ItemSyncer) Sync(ctx context.Context) error {
	s.logger.WithComponent("item_syncer").Info("starting item sync")

	// Fetch mappings from API
	mappings, err := s.client.GetItemMapping(ctx)
	if err != nil {
		s.logger.WithComponent("item_syncer").WithError(err).Error("failed to fetch item mappings")
		return err
	}

	s.logger.WithComponent("item_syncer").WithField("items_fetched", len(mappings)).Debug("fetched item mappings from API")

	// Upsert into database
	affected, err := s.repo.UpsertItems(ctx, mappings)
	if err != nil {
		s.logger.WithComponent("item_syncer").WithError(err).Error("failed to upsert items")
		return err
	}

	s.logger.WithComponent("item_syncer").WithFields(map[string]interface{}{
		"items_fetched": len(mappings),
		"rows_affected": affected,
	}).Info("item sync completed")

	return nil
}

// Start begins the item syncer with optional auto-refresh.
// Returns immediately after triggering initial sync (if configured).
// For periodic sync, call RunPeriodic in a goroutine.
func (s *ItemSyncer) Start(ctx context.Context) error {
	if s.config.SyncOnStart {
		if err := s.Sync(ctx); err != nil {
			return err
		}
	}
	return nil
}

// RunPeriodic runs periodic syncs according to config.SyncInterval.
// Blocks until context is cancelled.
func (s *ItemSyncer) RunPeriodic(ctx context.Context) {
	if s.config.SyncInterval <= 0 {
		return
	}

	ticker := time.NewTicker(s.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.Sync(ctx); err != nil {
				s.logger.WithComponent("item_syncer").WithError(err).Error("periodic sync failed")
			}
		}
	}
}
