package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// bucketTableName returns the table name for a given bucket size.
func bucketTableName(bucketSize string) string {
	switch bucketSize {
	case "5m":
		return "price_buckets_5m"
	case "1h":
		return "price_buckets_1h"
	case "24h":
		return "price_buckets_24h"
	default:
		return "price_buckets_5m" // fallback
	}
}

// LatestPrice represents the most recent price observation for an item.
type LatestPrice struct {
	ItemID    int
	HighPrice *int
	HighTime  *time.Time
	LowPrice  *int
	LowTime   *time.Time
}

// BucketMetrics represents aggregated volume/price data from a bucket table.
type BucketMetrics struct {
	ItemID          int
	AvgHighPrice    *int
	HighPriceVolume *int64
	AvgLowPrice     *int
	LowPriceVolume  *int64
}

// QueryRepository handles read operations for price data.
type QueryRepository struct {
	pool *pgxpool.Pool
}

// NewQueryRepository creates a new QueryRepository.
func NewQueryRepository(pool *pgxpool.Pool) *QueryRepository {
	return &QueryRepository{pool: pool}
}

// GetLatestPrices returns the most recent price observation for each item.
// Uses DISTINCT ON to get the latest observation per item_id.
func (r *QueryRepository) GetLatestPrices(ctx context.Context) ([]LatestPrice, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (item_id)
			item_id,
			high_price,
			high_time,
			low_price,
			low_time
		FROM price_observations
		ORDER BY item_id, observed_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query latest prices: %w", err)
	}
	defer rows.Close()

	var prices []LatestPrice
	for rows.Next() {
		var p LatestPrice
		if err := rows.Scan(&p.ItemID, &p.HighPrice, &p.HighTime, &p.LowPrice, &p.LowTime); err != nil {
			return nil, fmt.Errorf("scan price row: %w", err)
		}
		prices = append(prices, p)
	}

	return prices, rows.Err()
}

// GetLatestPricesForItems returns the most recent price observation for specific items.
func (r *QueryRepository) GetLatestPricesForItems(ctx context.Context, itemIDs []int) ([]LatestPrice, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (item_id)
			item_id,
			high_price,
			high_time,
			low_price,
			low_time
		FROM price_observations
		WHERE item_id = ANY($1)
		ORDER BY item_id, observed_at DESC
	`, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("query latest prices for items: %w", err)
	}
	defer rows.Close()

	var prices []LatestPrice
	for rows.Next() {
		var p LatestPrice
		if err := rows.Scan(&p.ItemID, &p.HighPrice, &p.HighTime, &p.LowPrice, &p.LowTime); err != nil {
			return nil, fmt.Errorf("scan price row: %w", err)
		}
		prices = append(prices, p)
	}

	return prices, rows.Err()
}

// GetVolumeMetrics returns aggregated volume metrics for items over a time range.
// bucketSize should be "5m", "1h", or "24h".
// duration specifies how far back to aggregate (e.g., 1 hour, 24 hours).
func (r *QueryRepository) GetVolumeMetrics(ctx context.Context, itemIDs []int, bucketSize string, duration time.Duration) (map[int]BucketMetrics, error) {
	if len(itemIDs) == 0 {
		return make(map[int]BucketMetrics), nil
	}

	tableName := bucketTableName(bucketSize)
	cutoff := time.Now().UTC().Add(-duration)

	// Aggregate volume and compute weighted average prices over the time range
	query := fmt.Sprintf(`
		SELECT
			item_id,
			CASE WHEN SUM(high_price_volume) > 0
				THEN SUM(avg_high_price::bigint * high_price_volume) / SUM(high_price_volume)
				ELSE NULL
			END as avg_high_price,
			SUM(high_price_volume) as high_price_volume,
			CASE WHEN SUM(low_price_volume) > 0
				THEN SUM(avg_low_price::bigint * low_price_volume) / SUM(low_price_volume)
				ELSE NULL
			END as avg_low_price,
			SUM(low_price_volume) as low_price_volume
		FROM %s
		WHERE item_id = ANY($1)
		  AND bucket_start >= $2
		GROUP BY item_id
	`, tableName)

	rows, err := r.pool.Query(ctx, query, itemIDs, cutoff)
	if err != nil {
		return nil, fmt.Errorf("query volume metrics from %s: %w", tableName, err)
	}
	defer rows.Close()

	result := make(map[int]BucketMetrics)
	for rows.Next() {
		var m BucketMetrics
		if err := rows.Scan(&m.ItemID, &m.AvgHighPrice, &m.HighPriceVolume, &m.AvgLowPrice, &m.LowPriceVolume); err != nil {
			return nil, fmt.Errorf("scan metrics row: %w", err)
		}
		result[m.ItemID] = m
	}

	return result, rows.Err()
}

// GetMultiPeriodVolumeMetrics returns volume metrics for multiple time periods.
// Returns metrics for 20m, 1h, and 24h periods.
func (r *QueryRepository) GetMultiPeriodVolumeMetrics(ctx context.Context, itemIDs []int) (map[int]*MultiPeriodMetrics, error) {
	if len(itemIDs) == 0 {
		return make(map[int]*MultiPeriodMetrics), nil
	}

	result := make(map[int]*MultiPeriodMetrics)
	for _, id := range itemIDs {
		result[id] = &MultiPeriodMetrics{}
	}

	// Get 20-minute metrics from 5m buckets
	metrics20m, err := r.GetVolumeMetrics(ctx, itemIDs, "5m", 20*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("get 20m metrics: %w", err)
	}
	for itemID, m := range metrics20m {
		if mp, ok := result[itemID]; ok {
			mp.Metrics20m = &m
		}
	}

	// Get 1-hour metrics from 1h buckets (or aggregate from 5m if more accurate)
	metrics1h, err := r.GetVolumeMetrics(ctx, itemIDs, "5m", 1*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("get 1h metrics: %w", err)
	}
	for itemID, m := range metrics1h {
		if mp, ok := result[itemID]; ok {
			mp.Metrics1h = &m
		}
	}

	// Get 24-hour metrics from 1h buckets
	metrics24h, err := r.GetVolumeMetrics(ctx, itemIDs, "1h", 24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("get 24h metrics: %w", err)
	}
	for itemID, m := range metrics24h {
		if mp, ok := result[itemID]; ok {
			mp.Metrics24h = &m
		}
	}

	return result, nil
}

// MultiPeriodMetrics holds metrics for multiple time periods.
type MultiPeriodMetrics struct {
	Metrics20m *BucketMetrics
	Metrics1h  *BucketMetrics
	Metrics24h *BucketMetrics
}

// GetDataFreshness returns the timestamp of the most recent observation.
// Returns nil if no data exists.
func (r *QueryRepository) GetDataFreshness(ctx context.Context) (*time.Time, error) {
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
		return nil, fmt.Errorf("query data freshness: %w", err)
	}
	return &t, nil
}

// IsDataFresh checks if the most recent observation is within the given threshold.
func (r *QueryRepository) IsDataFresh(ctx context.Context, threshold time.Duration) (bool, error) {
	freshness, err := r.GetDataFreshness(ctx)
	if err != nil {
		return false, err
	}
	if freshness == nil {
		return false, nil // No data at all
	}

	return time.Since(*freshness) <= threshold, nil
}

// GetItemCount returns the number of distinct items in observations.
func (r *QueryRepository) GetItemCount(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT item_id) FROM price_observations
	`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count items: %w", err)
	}
	return count, nil
}
