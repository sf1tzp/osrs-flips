package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PriceObservation represents a row in the price_observations table.
type PriceObservation struct {
	ItemID     int
	ObservedAt time.Time
	HighPrice  *int
	HighTime   *time.Time
	LowPrice   *int
	LowTime    *time.Time
}

// Repository handles database operations for the collector.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// InsertPriceObservations batch inserts price observations.
// Uses COPY for efficient bulk insertion.
func (r *Repository) InsertPriceObservations(ctx context.Context, observations []PriceObservation) (int64, error) {
	if len(observations) == 0 {
		return 0, nil
	}

	// Use COPY for bulk insert (much faster than individual INSERTs)
	columns := []string{"item_id", "observed_at", "high_price", "high_time", "low_price", "low_time"}

	copyCount, err := r.pool.CopyFrom(
		ctx,
		pgx.Identifier{"price_observations"},
		columns,
		pgx.CopyFromSlice(len(observations), func(i int) ([]interface{}, error) {
			obs := observations[i]
			return []interface{}{
				obs.ItemID,
				obs.ObservedAt,
				obs.HighPrice,
				obs.HighTime,
				obs.LowPrice,
				obs.LowTime,
			}, nil
		}),
	)
	if err != nil {
		return 0, fmt.Errorf("copy from: %w", err)
	}

	return copyCount, nil
}

// GetLatestObservationTime returns the most recent observation time, or nil if no data exists.
func (r *Repository) GetLatestObservationTime(ctx context.Context) (*time.Time, error) {
	var t time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT observed_at FROM price_observations
		ORDER BY observed_at DESC
		LIMIT 1
	`).Scan(&t)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query latest observation: %w", err)
	}
	return &t, nil
}

// GetObservationCount returns the total number of observations.
func (r *Repository) GetObservationCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM price_observations
	`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count observations: %w", err)
	}
	return count, nil
}

// PriceBucket represents a row in the price_buckets table.
type PriceBucket struct {
	ItemID          int
	BucketStart     time.Time
	BucketSize      string // "5m", "1h", "24h"
	AvgHighPrice    *int
	HighPriceVolume *int64
	AvgLowPrice     *int
	LowPriceVolume  *int64
	Source          string // "api" or "computed"
}

// InsertPriceBuckets batch inserts price buckets using upsert logic.
// On conflict, updates if the new data is from API (preferred over computed).
func (r *Repository) InsertPriceBuckets(ctx context.Context, buckets []PriceBucket) (int64, error) {
	if len(buckets) == 0 {
		return 0, nil
	}

	// Use batch insert with ON CONFLICT for upsert
	batch := &pgx.Batch{}
	for _, b := range buckets {
		batch.Queue(`
			INSERT INTO price_buckets (item_id, bucket_start, bucket_size, avg_high_price, high_price_volume, avg_low_price, low_price_volume, source)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (item_id, bucket_start, bucket_size) DO UPDATE SET
				avg_high_price = EXCLUDED.avg_high_price,
				high_price_volume = EXCLUDED.high_price_volume,
				avg_low_price = EXCLUDED.avg_low_price,
				low_price_volume = EXCLUDED.low_price_volume,
				source = EXCLUDED.source,
				ingested_at = NOW()
			WHERE price_buckets.source != 'api' OR EXCLUDED.source = 'api'
		`, b.ItemID, b.BucketStart, b.BucketSize, b.AvgHighPrice, b.HighPriceVolume, b.AvgLowPrice, b.LowPriceVolume, b.Source)
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()

	var inserted int64
	for range buckets {
		ct, err := br.Exec()
		if err != nil {
			return inserted, fmt.Errorf("batch exec: %w", err)
		}
		inserted += ct.RowsAffected()
	}

	return inserted, nil
}

// GetBucketCount returns the total number of buckets.
func (r *Repository) GetBucketCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM price_buckets`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count buckets: %w", err)
	}
	return count, nil
}

// GetBackfilledItems returns item IDs that have been backfilled for a given bucket size.
func (r *Repository) GetBackfilledItems(ctx context.Context, bucketSize string) (map[int]bool, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT item_id FROM price_buckets
		WHERE bucket_size = $1 AND source = 'api'
	`, bucketSize)
	if err != nil {
		return nil, fmt.Errorf("query backfilled items: %w", err)
	}
	defer rows.Close()

	items := make(map[int]bool)
	for rows.Next() {
		var itemID int
		if err := rows.Scan(&itemID); err != nil {
			return nil, fmt.Errorf("scan item id: %w", err)
		}
		items[itemID] = true
	}
	return items, rows.Err()
}

// GetDistinctItemIDs returns all distinct item IDs from price_observations.
func (r *Repository) GetDistinctItemIDs(ctx context.Context) ([]int, error) {
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT item_id FROM price_observations ORDER BY item_id`)
	if err != nil {
		return nil, fmt.Errorf("query distinct items: %w", err)
	}
	defer rows.Close()

	var items []int
	for rows.Next() {
		var itemID int
		if err := rows.Scan(&itemID); err != nil {
			return nil, fmt.Errorf("scan item id: %w", err)
		}
		items = append(items, itemID)
	}
	return items, rows.Err()
}
