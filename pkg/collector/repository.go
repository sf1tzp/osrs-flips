package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"osrs-flipping/pkg/osrs"
)

// Signal represents a trading signal stored in the signals table.
type Signal struct {
	ItemID     int
	SignalType string
	Score      float64
	Metadata   map[string]interface{}
	ExpiresAt  time.Time
}

// SpreadCandidate holds data for a single item's spread widening evaluation.
type SpreadCandidate struct {
	ItemID        int
	ItemName      string
	Icon          string
	HighPrice     int
	LowPrice      int
	CurrentSpread int
	AvgSpread     float64
	BuyLimit      *int
}

// InversionCandidate holds data for a price inversion evaluation.
// A price inversion occurs when high_price < low_price, suggesting bot dumping or panic selling.
type InversionCandidate struct {
	ItemID         int
	ItemName       string
	Icon           string
	HighPrice      int
	LowPrice       int
	Spread         int     // high - low (negative = inverted)
	Volume         int64   // recent high_price_volume from 5m buckets
	BaselineVolume float64 // 24h average high_price_volume
	VolumeRatio    float64 // volume / baseline
	BuyLimit       *int
}

// PricePoint is a single 1h bucket price for time-series indicator computation.
type PricePoint struct {
	Time  time.Time
	Price int // avg_low_price (insta-sell / buy price)
}

// ItemPriceSeries holds the 1h price series for an item, used by MACD/RSI.
type ItemPriceSeries struct {
	ItemID   int
	ItemName string
	Icon     string
	BuyLimit *int
	Prices   []PricePoint
}

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
// Processes in chunks of 1000 to avoid unbounded server-side batch buffering.
func (r *Repository) insertBucketsToTable(ctx context.Context, tableName string, buckets []PriceBucket) (int64, error) {
	const chunkSize = 1000
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

	var inserted int64

	for i := 0; i < len(buckets); i += chunkSize {
		end := i + chunkSize
		if end > len(buckets) {
			end = len(buckets)
		}
		chunk := buckets[i:end]

		batch := &pgx.Batch{}
		for _, b := range chunk {
			batch.Queue(query, b.ItemID, b.BucketStart, b.AvgHighPrice, b.HighPriceVolume, b.AvgLowPrice, b.LowPriceVolume, b.Source)
		}

		br := r.pool.SendBatch(ctx, batch)
		for range chunk {
			ct, err := br.Exec()
			if err != nil {
				br.Close()
				return inserted, fmt.Errorf("batch exec: %w", err)
			}
			inserted += ct.RowsAffected()
		}
		br.Close()
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
// Also marks the source as 'manual' so auto-disable never clobbers user intent.
func (r *Repository) SetPollVolume(ctx context.Context, itemIDs []int, pollVolume bool) (int64, error) {
	if len(itemIDs) == 0 {
		return 0, nil
	}

	query := `UPDATE items SET poll_volume = $1, updated_at = NOW() WHERE item_id = ANY($2)`
	if pollVolume {
		query = `UPDATE items SET poll_volume = TRUE, poll_volume_source = 'manual', updated_at = NOW() WHERE item_id = ANY($1)`
		ct, err := r.pool.Exec(ctx, query, itemIDs)
		if err != nil {
			return 0, fmt.Errorf("update poll volume: %w", err)
		}
		return ct.RowsAffected(), nil
	}

	ct, err := r.pool.Exec(ctx, query, pollVolume, itemIDs)
	if err != nil {
		return 0, fmt.Errorf("update poll volume: %w", err)
	}
	return ct.RowsAffected(), nil
}

// VolumeRatio holds recent vs baseline volume data for an item.
type VolumeRatio struct {
	Recent5m  float64
	Avg24h    float64
	Ratio     float64
}

// GetCompoundedItemIDs returns item IDs that have 2+ active (non-expired) signals.
func (r *Repository) GetCompoundedItemIDs(ctx context.Context) ([]int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT item_id
		FROM signals
		WHERE expires_at > NOW()
		GROUP BY item_id
		HAVING COUNT(*) >= 2
		ORDER BY item_id
	`)
	if err != nil {
		return nil, fmt.Errorf("query compounded item ids: %w", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan compounded item id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SetPollVolumeAuto enables poll_volume for items, setting source='auto'.
// Skips items already set to source='manual' to protect user overrides.
// Returns the number of rows updated.
func (r *Repository) SetPollVolumeAuto(ctx context.Context, itemIDs []int) (int64, error) {
	if len(itemIDs) == 0 {
		return 0, nil
	}

	ct, err := r.pool.Exec(ctx, `
		UPDATE items
		SET poll_volume = TRUE,
		    poll_volume_source = 'auto',
		    poll_volume_auto_at = NOW(),
		    updated_at = NOW()
		WHERE item_id = ANY($1)
		  AND poll_volume_source != 'manual'
	`, itemIDs)
	if err != nil {
		return 0, fmt.Errorf("set poll volume auto: %w", err)
	}
	return ct.RowsAffected(), nil
}

// DisableAutoPolledVolume disables poll_volume for auto-enabled items that are
// no longer in keepItemIDs, respecting a grace period before disabling.
func (r *Repository) DisableAutoPolledVolume(ctx context.Context, keepItemIDs []int, gracePeriod time.Duration) (int64, error) {
	ct, err := r.pool.Exec(ctx, `
		UPDATE items
		SET poll_volume = FALSE,
		    poll_volume_source = 'manual',
		    poll_volume_auto_at = NULL,
		    updated_at = NOW()
		WHERE poll_volume = TRUE
		  AND poll_volume_source = 'auto'
		  AND (CARDINALITY($1::int[]) = 0 OR item_id != ALL($1::int[]))
		  AND poll_volume_auto_at < NOW() - $2::interval
	`, keepItemIDs, gracePeriod)
	if err != nil {
		return 0, fmt.Errorf("disable auto polled volume: %w", err)
	}
	return ct.RowsAffected(), nil
}

// GetVolumeRatios returns recent_5m_volume / avg_24h_volume for given items.
// Uses high_price_volume from price_buckets_5m.
func (r *Repository) GetVolumeRatios(ctx context.Context, itemIDs []int) (map[int]VolumeRatio, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}

	rows, err := r.pool.Query(ctx, `
		WITH recent AS (
			SELECT DISTINCT ON (item_id)
				item_id,
				COALESCE(high_price_volume, 0) AS vol
			FROM price_buckets_5m
			WHERE item_id = ANY($1)
			  AND high_price_volume IS NOT NULL
			ORDER BY item_id, bucket_start DESC
		),
		baseline AS (
			SELECT item_id,
			       AVG(high_price_volume) AS avg_vol
			FROM price_buckets_5m
			WHERE item_id = ANY($1)
			  AND bucket_start >= NOW() - INTERVAL '24 hours'
			  AND high_price_volume IS NOT NULL
			GROUP BY item_id
			HAVING AVG(high_price_volume) > 0
		)
		SELECT r.item_id, r.vol, b.avg_vol, r.vol::float / b.avg_vol AS ratio
		FROM recent r
		JOIN baseline b ON r.item_id = b.item_id
	`, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("query volume ratios: %w", err)
	}
	defer rows.Close()

	result := make(map[int]VolumeRatio)
	for rows.Next() {
		var itemID int
		var vr VolumeRatio
		if err := rows.Scan(&itemID, &vr.Recent5m, &vr.Avg24h, &vr.Ratio); err != nil {
			return nil, fmt.Errorf("scan volume ratio: %w", err)
		}
		result[itemID] = vr
	}
	return result, rows.Err()
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

// bucketDuration returns the time.Duration for a bucket size.
func bucketDuration(bucketSize string) time.Duration {
	switch bucketSize {
	case "5m":
		return 5 * time.Minute
	case "1h":
		return time.Hour
	case "24h":
		return 24 * time.Hour
	default:
		return 5 * time.Minute
	}
}

// GetMissingBucketTimestamps returns timestamps in the retention window that have
// never been fetched successfully. Results are ordered newest-first so recent gaps
// (most likely to have API data) get priority.
//
// Approach: generate candidate timestamps in Go, send them to the DB in chunks
// using WHERE bucket_start = ANY($1) to check which exist. This avoids the
// full-table DISTINCT that caused OOM on millions of rows.
func (r *Repository) GetMissingBucketTimestamps(ctx context.Context, bucketSize string, retention time.Duration, limit int) ([]time.Time, error) {
	tableName := bucketTableName(bucketSize)
	interval := bucketDuration(bucketSize)

	now := time.Now().UTC()
	var windowStart time.Time
	if retention > 0 {
		windowStart = now.Add(-retention)
	} else {
		windowStart = now.AddDate(-1, 0, 0)
	}
	// Exclude the current incomplete bucket
	windowEnd := now.Add(-interval)

	// Align windowStart up to the next bucket boundary
	aligned := windowStart.Truncate(interval)
	if aligned.Before(windowStart) {
		aligned = aligned.Add(interval)
	}

	// Generate all candidate timestamps (newest first)
	end := windowEnd.Truncate(interval)
	var candidates []time.Time
	for ts := end; !ts.Before(aligned); ts = ts.Add(-interval) {
		candidates = append(candidates, ts)
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	// Query existing timestamps in chunks of 1000 to avoid large parameter lists
	const chunkSize = 1000
	skip := make(map[time.Time]bool, len(candidates))

	for i := 0; i < len(candidates); i += chunkSize {
		end := i + chunkSize
		if end > len(candidates) {
			end = len(candidates)
		}
		chunk := candidates[i:end]

		// Check bucket table for existing data
		query := fmt.Sprintf(
			`SELECT DISTINCT bucket_start FROM %s WHERE bucket_start = ANY($1)`,
			tableName,
		)
		rows, err := r.pool.Query(ctx, query, chunk)
		if err != nil {
			return nil, fmt.Errorf("query existing bucket timestamps: %w", err)
		}
		for rows.Next() {
			var ts time.Time
			if err := rows.Scan(&ts); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan bucket timestamp: %w", err)
			}
			skip[ts.UTC()] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate bucket timestamps: %w", err)
		}

		// Check no-data markers
		rows, err = r.pool.Query(ctx,
			`SELECT bucket_start FROM sync_no_data WHERE bucket_size = $1 AND bucket_start = ANY($2)`,
			bucketSize, chunk,
		)
		if err != nil {
			return nil, fmt.Errorf("query no-data timestamps: %w", err)
		}
		for rows.Next() {
			var ts time.Time
			if err := rows.Scan(&ts); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan no-data timestamp: %w", err)
			}
			skip[ts.UTC()] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate no-data timestamps: %w", err)
		}
	}

	// Filter candidates to only missing ones (already in newest-first order)
	var missing []time.Time
	for _, ts := range candidates {
		if !skip[ts] {
			missing = append(missing, ts)
			if len(missing) >= limit {
				break
			}
		}
	}

	return missing, nil
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

// SyncCoverageStats holds coverage statistics for a bucket size.
type SyncCoverageStats struct {
	TotalItems      int64
	ItemsWithData   int64
	ItemsWithNoData int64
	OldestBucket    *time.Time
	NewestBucket    *time.Time
}

// GetSyncCoverageStats returns coverage statistics for a given bucket size.
func (r *Repository) GetSyncCoverageStats(ctx context.Context, bucketSize string) (*SyncCoverageStats, error) {
	tableName := bucketTableName(bucketSize)

	var stats SyncCoverageStats

	// Get total items count
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM items`).Scan(&stats.TotalItems); err != nil {
		return nil, fmt.Errorf("count items: %w", err)
	}

	// Get items with data count
	query := fmt.Sprintf(`SELECT COUNT(DISTINCT item_id) FROM %s`, tableName)
	if err := r.pool.QueryRow(ctx, query).Scan(&stats.ItemsWithData); err != nil {
		return nil, fmt.Errorf("count items with data: %w", err)
	}

	stats.ItemsWithNoData = stats.TotalItems - stats.ItemsWithData

	// Get oldest and newest bucket timestamps
	query = fmt.Sprintf(`SELECT MIN(bucket_start), MAX(bucket_start) FROM %s`, tableName)
	if err := r.pool.QueryRow(ctx, query).Scan(&stats.OldestBucket, &stats.NewestBucket); err != nil {
		return nil, fmt.Errorf("get bucket range: %w", err)
	}

	return &stats, nil
}

// CompletenessDistribution holds the count of items in each completeness bracket.
type CompletenessDistribution struct {
	Complete90Plus int64 // 90%+ complete
	Complete50to89 int64 // 50-89% complete
	Complete10to49 int64 // 10-49% complete
	CompleteLt10   int64 // <10% complete (but >0)
	CompleteZero   int64 // 0% complete (no data)
}

// GetCompletenessDistribution returns the distribution of item completeness for a bucket size.
func (r *Repository) GetCompletenessDistribution(ctx context.Context, bucketSize string, retention time.Duration) (*CompletenessDistribution, error) {
	tableName := bucketTableName(bucketSize)

	// Calculate the retention window start
	var windowStart time.Time
	if retention > 0 {
		windowStart = time.Now().UTC().Add(-retention)
	} else {
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

	query := fmt.Sprintf(`
		WITH expected AS (
			SELECT EXTRACT(EPOCH FROM (NOW() - $1::timestamptz)) / EXTRACT(EPOCH FROM $2::interval) as expected_buckets
		),
		bucket_counts AS (
			SELECT item_id, COUNT(*) as actual_buckets
			FROM %s
			WHERE bucket_start > $1
			GROUP BY item_id
		),
		completeness AS (
			SELECT
				i.item_id,
				COALESCE(b.actual_buckets, 0) as actual,
				e.expected_buckets as expected,
				CASE
					WHEN e.expected_buckets = 0 THEN 0
					ELSE COALESCE(b.actual_buckets, 0)::float / e.expected_buckets * 100
				END as pct
			FROM items i
			CROSS JOIN expected e
			LEFT JOIN bucket_counts b ON i.item_id = b.item_id
		)
		SELECT
			COUNT(*) FILTER (WHERE pct >= 90) as complete_90plus,
			COUNT(*) FILTER (WHERE pct >= 50 AND pct < 90) as complete_50to89,
			COUNT(*) FILTER (WHERE pct >= 10 AND pct < 50) as complete_10to49,
			COUNT(*) FILTER (WHERE pct > 0 AND pct < 10) as complete_lt10,
			COUNT(*) FILTER (WHERE pct = 0) as complete_zero
		FROM completeness
	`, tableName)

	var dist CompletenessDistribution
	err := r.pool.QueryRow(ctx, query, windowStart, interval).Scan(
		&dist.Complete90Plus,
		&dist.Complete50to89,
		&dist.Complete10to49,
		&dist.CompleteLt10,
		&dist.CompleteZero,
	)
	if err != nil {
		return nil, fmt.Errorf("get completeness distribution: %w", err)
	}

	return &dist, nil
}

// ItemWithZeroData represents an item with no bucket data.
type ItemWithZeroData struct {
	ItemID int
	Name   string
}

// GetItemsWithZeroData returns items that have no bucket data for the given bucket size.
func (r *Repository) GetItemsWithZeroData(ctx context.Context, bucketSize string, limit int) ([]ItemWithZeroData, error) {
	tableName := bucketTableName(bucketSize)

	query := fmt.Sprintf(`
		SELECT i.item_id, i.name
		FROM items i
		LEFT JOIN (SELECT DISTINCT item_id FROM %s) b ON i.item_id = b.item_id
		WHERE b.item_id IS NULL
		ORDER BY i.item_id
		LIMIT $1
	`, tableName)

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query items with zero data: %w", err)
	}
	defer rows.Close()

	var items []ItemWithZeroData
	for rows.Next() {
		var item ItemWithZeroData
		if err := rows.Scan(&item.ItemID, &item.Name); err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// RecordNoData records that the API returned no data for a given bucket timestamp,
// preventing it from being retried on subsequent sync cycles.
func (r *Repository) RecordNoData(ctx context.Context, bucketSize string, bucketStart time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO sync_no_data (bucket_size, bucket_start)
		VALUES ($1, $2)
		ON CONFLICT (bucket_size, bucket_start) DO NOTHING
	`, bucketSize, bucketStart)
	if err != nil {
		return fmt.Errorf("record no data: %w", err)
	}
	return nil
}

// getPriceSeries returns items with at least minBuckets consecutive 1h buckets
// of non-null avg_low_price data. Results are grouped by item with prices ordered ASC.
func (r *Repository) getPriceSeries(ctx context.Context, minBuckets int) ([]ItemPriceSeries, error) {
	rows, err := r.pool.Query(ctx, `
		WITH eligible AS (
			SELECT item_id
			FROM price_buckets_1h
			WHERE bucket_start >= NOW() - INTERVAL '48 hours'
			  AND avg_low_price IS NOT NULL
			GROUP BY item_id
			HAVING COUNT(*) >= $1
		)
		SELECT b.item_id, i.name, i.icon, i.buy_limit,
		       b.bucket_start, b.avg_low_price
		FROM price_buckets_1h b
		JOIN eligible e ON b.item_id = e.item_id
		JOIN items i ON b.item_id = i.item_id
		WHERE b.bucket_start >= NOW() - INTERVAL '48 hours'
		  AND b.avg_low_price IS NOT NULL
		ORDER BY b.item_id, b.bucket_start ASC
	`, minBuckets)
	if err != nil {
		return nil, fmt.Errorf("query price series: %w", err)
	}
	defer rows.Close()

	var result []ItemPriceSeries
	var current *ItemPriceSeries

	for rows.Next() {
		var itemID int
		var name string
		var icon *string
		var buyLimit *int
		var bucketStart time.Time
		var avgLowPrice int

		if err := rows.Scan(&itemID, &name, &icon, &buyLimit, &bucketStart, &avgLowPrice); err != nil {
			return nil, fmt.Errorf("scan price series row: %w", err)
		}

		if current == nil || current.ItemID != itemID {
			if current != nil {
				result = append(result, *current)
			}
			current = &ItemPriceSeries{
				ItemID:   itemID,
				ItemName: name,
				BuyLimit: buyLimit,
			}
			if icon != nil {
				current.Icon = *icon
			}
		}
		current.Prices = append(current.Prices, PricePoint{
			Time:  bucketStart,
			Price: avgLowPrice,
		})
	}
	if current != nil {
		result = append(result, *current)
	}

	return result, rows.Err()
}

// GetPriceSeries returns items with at least minBuckets consecutive 1h buckets
// of non-null avg_low_price data. Public wrapper around getPriceSeries.
func (r *Repository) GetPriceSeries(ctx context.Context, minBuckets int) ([]ItemPriceSeries, error) {
	return r.getPriceSeries(ctx, minBuckets)
}

// GetSpreadWideningCandidates returns items whose current spread exceeds 1.5x the
// 24-hour rolling average spread (computed from 1h buckets).
func (r *Repository) GetSpreadWideningCandidates(ctx context.Context) ([]SpreadCandidate, error) {
	rows, err := r.pool.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (item_id) item_id, high_price, low_price
			FROM price_observations
			WHERE high_price IS NOT NULL AND low_price IS NOT NULL
			ORDER BY item_id, observed_at DESC
		),
		avg_spread AS (
			SELECT item_id,
			       AVG(COALESCE(avg_high_price, 0) - COALESCE(avg_low_price, 0)) AS avg_spread
			FROM price_buckets_1h
			WHERE bucket_start >= NOW() - INTERVAL '24 hours'
			  AND avg_high_price IS NOT NULL AND avg_low_price IS NOT NULL
			GROUP BY item_id
			HAVING AVG(COALESCE(avg_high_price, 0) - COALESCE(avg_low_price, 0)) > 0
		)
		SELECT l.item_id, i.name, i.icon, i.buy_limit,
		       l.high_price, l.low_price,
		       (l.high_price - l.low_price) AS current_spread,
		       a.avg_spread
		FROM latest l
		JOIN avg_spread a ON l.item_id = a.item_id
		JOIN items i ON l.item_id = i.item_id
		WHERE (l.high_price - l.low_price) > a.avg_spread * 1.5
	`)
	if err != nil {
		return nil, fmt.Errorf("query spread widening candidates: %w", err)
	}
	defer rows.Close()

	var candidates []SpreadCandidate
	for rows.Next() {
		var c SpreadCandidate
		var icon *string
		if err := rows.Scan(&c.ItemID, &c.ItemName, &icon, &c.BuyLimit,
			&c.HighPrice, &c.LowPrice, &c.CurrentSpread, &c.AvgSpread); err != nil {
			return nil, fmt.Errorf("scan spread candidate: %w", err)
		}
		if icon != nil {
			c.Icon = *icon
		}
		candidates = append(candidates, c)
	}
	return candidates, rows.Err()
}

// GetInversionCandidates returns items where high_price < low_price (inverted spread)
// with a volume spike relative to their 24h baseline volume from 5m buckets.
// Only considers items with poll_volume=true (VolumePoller active).
func (r *Repository) GetInversionCandidates(ctx context.Context) ([]InversionCandidate, error) {
	rows, err := r.pool.Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (item_id) item_id, high_price, low_price
			FROM price_observations
			WHERE high_price IS NOT NULL AND low_price IS NOT NULL
			ORDER BY item_id, observed_at DESC
		),
		recent_volume AS (
			-- Most recent 5m bucket volume per item (the current window)
			SELECT DISTINCT ON (item_id)
				item_id, high_price_volume
			FROM price_buckets_5m
			WHERE high_price_volume IS NOT NULL
			ORDER BY item_id, bucket_start DESC
		),
		baseline_volume AS (
			-- Average 5m high_price_volume over the last 24h per item
			SELECT item_id,
			       AVG(high_price_volume) AS avg_volume
			FROM price_buckets_5m
			WHERE bucket_start >= NOW() - INTERVAL '24 hours'
			  AND high_price_volume IS NOT NULL
			GROUP BY item_id
			HAVING AVG(high_price_volume) > 0
		)
		SELECT l.item_id, i.name, i.icon, i.buy_limit,
		       l.high_price, l.low_price,
		       (l.high_price - l.low_price) AS spread,
		       rv.high_price_volume AS volume,
		       bv.avg_volume AS baseline_volume,
		       rv.high_price_volume::float / bv.avg_volume AS volume_ratio
		FROM latest l
		JOIN items i ON l.item_id = i.item_id
		JOIN recent_volume rv ON l.item_id = rv.item_id
		JOIN baseline_volume bv ON l.item_id = bv.item_id
		WHERE i.poll_volume = TRUE
		  AND l.high_price < l.low_price
		  AND rv.high_price_volume::float / bv.avg_volume >= 3.0
	`)
	if err != nil {
		return nil, fmt.Errorf("query inversion candidates: %w", err)
	}
	defer rows.Close()

	var candidates []InversionCandidate
	for rows.Next() {
		var c InversionCandidate
		var icon *string
		if err := rows.Scan(&c.ItemID, &c.ItemName, &icon, &c.BuyLimit,
			&c.HighPrice, &c.LowPrice, &c.Spread,
			&c.Volume, &c.BaselineVolume, &c.VolumeRatio); err != nil {
			return nil, fmt.Errorf("scan inversion candidate: %w", err)
		}
		if icon != nil {
			c.Icon = *icon
		}
		candidates = append(candidates, c)
	}
	return candidates, rows.Err()
}

// UpsertSignals batch upserts signals using ON CONFLICT on (item_id, signal_type).
// Processes in chunks of 500 to avoid unbounded server-side buffering.
func (r *Repository) UpsertSignals(ctx context.Context, signals []Signal) (int64, error) {
	if len(signals) == 0 {
		return 0, nil
	}

	const chunkSize = 500
	var upserted int64

	for i := 0; i < len(signals); i += chunkSize {
		end := i + chunkSize
		if end > len(signals) {
			end = len(signals)
		}
		chunk := signals[i:end]

		batch := &pgx.Batch{}
		for _, s := range chunk {
			metaJSON, err := json.Marshal(s.Metadata)
			if err != nil {
				return upserted, fmt.Errorf("marshal signal metadata: %w", err)
			}
			batch.Queue(`
				INSERT INTO signals (item_id, signal_type, score, metadata, expires_at)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (item_id, signal_type) DO UPDATE SET
					score = EXCLUDED.score,
					metadata = EXCLUDED.metadata,
					created_at = NOW(),
					expires_at = EXCLUDED.expires_at
			`, s.ItemID, s.SignalType, s.Score, metaJSON, s.ExpiresAt)
		}

		br := r.pool.SendBatch(ctx, batch)
		for range chunk {
			ct, err := br.Exec()
			if err != nil {
				br.Close()
				return upserted, fmt.Errorf("batch exec signal upsert: %w", err)
			}
			upserted += ct.RowsAffected()
		}
		br.Close()
	}

	return upserted, nil
}

// DeleteExpiredSignals removes signals whose expiry has passed.
func (r *Repository) DeleteExpiredSignals(ctx context.Context) (int64, error) {
	ct, err := r.pool.Exec(ctx, `DELETE FROM signals WHERE expires_at < NOW()`)
	if err != nil {
		return 0, fmt.Errorf("delete expired signals: %w", err)
	}
	return ct.RowsAffected(), nil
}
