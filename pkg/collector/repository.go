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
