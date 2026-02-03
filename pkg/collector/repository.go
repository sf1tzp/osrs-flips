package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"osrs-flipping/pkg/osrs"
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

// PriceBucket represents a row in the price_buckets_* tables.
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

// InsertPriceBuckets batch inserts price buckets using upsert logic.
// Routes to the appropriate table based on bucket size.
// On conflict, updates if the new data is from API (preferred over computed).
func (r *Repository) InsertPriceBuckets(ctx context.Context, buckets []PriceBucket) (int64, error) {
	if len(buckets) == 0 {
		return 0, nil
	}

	// Group buckets by size for efficient batch operations
	bySize := make(map[string][]PriceBucket)
	for _, b := range buckets {
		bySize[b.BucketSize] = append(bySize[b.BucketSize], b)
	}

	var totalInserted int64
	for bucketSize, sizeBuckets := range bySize {
		tableName := bucketTableName(bucketSize)
		inserted, err := r.insertBucketsToTable(ctx, tableName, sizeBuckets)
		if err != nil {
			return totalInserted, fmt.Errorf("insert to %s: %w", tableName, err)
		}
		totalInserted += inserted
	}

	return totalInserted, nil
}

// insertBucketsToTable inserts buckets to a specific table.
func (r *Repository) insertBucketsToTable(ctx context.Context, tableName string, buckets []PriceBucket) (int64, error) {
	batch := &pgx.Batch{}
	for _, b := range buckets {
		// Note: table name is from our controlled bucketTableName(), not user input
		query := fmt.Sprintf(`
			INSERT INTO %s (item_id, bucket_start, avg_high_price, high_price_volume, avg_low_price, low_price_volume, source)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (item_id, bucket_start) DO UPDATE SET
				avg_high_price = EXCLUDED.avg_high_price,
				high_price_volume = EXCLUDED.high_price_volume,
				avg_low_price = EXCLUDED.avg_low_price,
				low_price_volume = EXCLUDED.low_price_volume,
				source = EXCLUDED.source,
				ingested_at = NOW()
			WHERE %s.source != 'api' OR EXCLUDED.source = 'api'
		`, tableName, tableName)
		batch.Queue(query, b.ItemID, b.BucketStart, b.AvgHighPrice, b.HighPriceVolume, b.AvgLowPrice, b.LowPriceVolume, b.Source)
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

// GetBucketCount returns the total number of buckets for a given bucket size.
func (r *Repository) GetBucketCount(ctx context.Context, bucketSize string) (int64, error) {
	tableName := bucketTableName(bucketSize)
	var count int64
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, tableName)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count buckets: %w", err)
	}
	return count, nil
}

// Item represents a row in the items table.
type Item struct {
	ItemID     int
	Name       string
	Examine    string
	Members    bool
	BuyLimit   *int
	HighAlch   *int
	LowAlch    *int
	GEValue    *int
	Icon       string
	PollVolume bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// UpsertItems batch upserts items from API mapping data.
// Returns the number of rows affected.
func (r *Repository) UpsertItems(ctx context.Context, mappings []osrs.ItemMapping) (int64, error) {
	if len(mappings) == 0 {
		return 0, nil
	}

	batch := &pgx.Batch{}
	for _, m := range mappings {
		query := `
			INSERT INTO items (item_id, name, examine, members, buy_limit, high_alch, low_alch, ge_value, icon, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
			ON CONFLICT (item_id) DO UPDATE SET
				name = EXCLUDED.name,
				examine = EXCLUDED.examine,
				members = EXCLUDED.members,
				buy_limit = EXCLUDED.buy_limit,
				high_alch = EXCLUDED.high_alch,
				low_alch = EXCLUDED.low_alch,
				ge_value = EXCLUDED.ge_value,
				icon = EXCLUDED.icon,
				updated_at = NOW()
		`
		// Convert zero values to nil for optional fields
		var buyLimit, highAlch, lowAlch, geValue *int
		if m.BuyLimit > 0 {
			buyLimit = &m.BuyLimit
		}
		if m.HighAlch > 0 {
			highAlch = &m.HighAlch
		}
		if m.LowAlch > 0 {
			lowAlch = &m.LowAlch
		}
		if m.Value > 0 {
			geValue = &m.Value
		}
		batch.Queue(query, m.ID, m.Name, m.Examine, m.Members, buyLimit, highAlch, lowAlch, geValue, m.Icon)
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()

	var affected int64
	for range mappings {
		ct, err := br.Exec()
		if err != nil {
			return affected, fmt.Errorf("batch exec: %w", err)
		}
		affected += ct.RowsAffected()
	}

	return affected, nil
}

// GetItemsToPollVolume returns item IDs that have poll_volume=true.
func (r *Repository) GetItemsToPollVolume(ctx context.Context) ([]int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT item_id FROM items WHERE poll_volume = TRUE ORDER BY item_id
	`)
	if err != nil {
		return nil, fmt.Errorf("query poll volume items: %w", err)
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

// SetPollVolume sets the poll_volume flag for specified item IDs.
func (r *Repository) SetPollVolume(ctx context.Context, itemIDs []int, pollVolume bool) (int64, error) {
	if len(itemIDs) == 0 {
		return 0, nil
	}

	ct, err := r.pool.Exec(ctx, `
		UPDATE items SET poll_volume = $1, updated_at = NOW()
		WHERE item_id = ANY($2)
	`, pollVolume, itemIDs)
	if err != nil {
		return 0, fmt.Errorf("update poll volume: %w", err)
	}
	return ct.RowsAffected(), nil
}

// GetItemCount returns the total number of items in the items table.
func (r *Repository) GetItemCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM items`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count items: %w", err)
	}
	return count, nil
}

// GetItemsNeedingSync returns item IDs that need historical data sync.
// This includes both:
// - Items with no bucket data for the given bucket size (new items)
// - Items with gaps (incomplete coverage) within the retention window
// Items are prioritized by recent observation activity, then by item_id.
// retention=0 means no retention limit (uses 1 year lookback).
func (r *Repository) GetItemsNeedingSync(ctx context.Context, bucketSize string, retention time.Duration, limit int) ([]int, error) {
	tableName := bucketTableName(bucketSize)

	// Calculate the retention window
	var windowStart time.Time
	if retention > 0 {
		windowStart = time.Now().UTC().Add(-retention)
	} else {
		// For unlimited retention, use a reasonable lookback (1 year)
		windowStart = time.Now().UTC().AddDate(-1, 0, 0)
	}

	// Calculate expected bucket interval
	var interval string
	switch bucketSize {
	case "5m":
		interval = "5 minutes"
	case "1h":
		interval = "1 hour"
	case "24h":
		interval = "24 hours"
	default:
		interval = "5 minutes"
	}

	// Query finds items that need sync:
	// 1. Source from items table (all known items)
	// 2. Left join to bucket counts within retention window
	// 3. Left join to price_observations for activity-based prioritization
	// 4. Filter to items where actual_buckets < expected_buckets * 0.9 (10% tolerance)
	// 5. Order by recent activity (nulls last), then by item_id for determinism
	query := fmt.Sprintf(`
		WITH bucket_counts AS (
			-- Count actual buckets per item in the retention window
			SELECT item_id, COUNT(*) as actual_buckets
			FROM %s
			WHERE bucket_start > $1
			GROUP BY item_id
		),
		recent_activity AS (
			-- Get most recent observation per item for prioritization
			SELECT item_id, MAX(observed_at) as last_seen
			FROM price_observations
			WHERE observed_at > $1
			GROUP BY item_id
		),
		expected AS (
			-- Calculate expected bucket count for the window
			SELECT EXTRACT(EPOCH FROM (NOW() - $1::timestamptz)) / EXTRACT(EPOCH FROM $2::interval) as expected_buckets
		)
		SELECT i.item_id
		FROM items i
		LEFT JOIN bucket_counts b ON i.item_id = b.item_id
		LEFT JOIN recent_activity r ON i.item_id = r.item_id
		CROSS JOIN expected e
		WHERE COALESCE(b.actual_buckets, 0) < e.expected_buckets * 0.9
		ORDER BY r.last_seen DESC NULLS LAST, i.item_id
		LIMIT $3
	`, tableName)

	rows, err := r.pool.Query(ctx, query, windowStart, interval, limit)
	if err != nil {
		return nil, fmt.Errorf("query items needing sync: %w", err)
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

// GetItem returns a single item by ID.
func (r *Repository) GetItem(ctx context.Context, itemID int) (*Item, error) {
	var item Item
	err := r.pool.QueryRow(ctx, `
		SELECT item_id, name, examine, members, buy_limit, high_alch, low_alch, ge_value, icon, poll_volume, created_at, updated_at
		FROM items WHERE item_id = $1
	`, itemID).Scan(
		&item.ItemID, &item.Name, &item.Examine, &item.Members,
		&item.BuyLimit, &item.HighAlch, &item.LowAlch, &item.GEValue,
		&item.Icon, &item.PollVolume, &item.CreatedAt, &item.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query item: %w", err)
	}
	return &item, nil
}
