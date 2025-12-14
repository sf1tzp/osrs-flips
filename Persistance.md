# High-Level Reference: Price Data System

## API Structure

**Three complementary endpoints:**

1. **`/latest`** - Current snapshot for all items (single call)
   - Returns: `{item_id: {high, highTime, low, lowTime}}`
   - Use: Real-time polling

2. **`/timeseries?timestep={5m|1h|24h}&id={item_id}`** - Historical aggregates per item
   - Returns: 365 buckets of `{timestamp, avgHighPrice, avgLowPrice, highPriceVolume, lowPriceVolume}`
   - Use: Historical backfill, trend analysis

3. **`/5m?timestamp={ts}`** - Point-in-time snapshot for all items at specific bucket
   - Returns: `{item_id: {avgHighPrice, avgLowPrice, highPriceVolume, lowPriceVolume}}`
   - Use: Surgical gap filling

## Database Schema (PostgreSQL + TimescaleDB)

**Two core tables:**

```sql
-- High-resolution raw data from /latest polling
price_observations (
    item_id, observed_at,
    high_price, high_time, low_price, low_time
    PRIMARY KEY (item_id, observed_at)
) PARTITION BY RANGE (observed_at)

-- Pre-aggregated bucket data from API
price_buckets (
    item_id, bucket_start, bucket_size,
    avg_high_price, high_price_volume,
    avg_low_price, low_price_volume,
    source, ingested_at
    PRIMARY KEY (item_id, bucket_start, bucket_size)
) PARTITION BY RANGE (bucket_start)
```

**Key design decisions:**
- Separate raw observations (30-60s polling) from bucketed aggregates (5m/1h/24h)
- Track `source` field ('api' vs 'computed') for data provenance
- Time-based partitioning for efficient pruning
- Keep raw data 7-30 days, rely on buckets for historical analysis

## Ingestion Strategy

**Historical Backfill:**
- Use `/timeseries` endpoint per item (24h → 1h → 5m buckets)
- Work backwards from present, rate-limited
- Populate `price_buckets` with source='api'

**Real-time Continuous:**
- Poll `/latest` every 30-60 seconds → `price_observations`
- Single API call captures all 4000 items
- Enables sub-bucket resolution and immediate alerting

**Gap Filling:**
- Hourly: Check for missing 5m buckets in last 24h, fill via `/5m` API
- Daily: Backfill missing larger buckets, prune old raw observations
- Compute own buckets from raw data, compare against API for validation

**Data Lifecycle:**
- Raw observations: Keep 7-30 days for high-resolution analysis
- Bucket aggregates: Keep indefinitely for historical trends
- Prefer API buckets (include volume data) over computed when available

## Query Patterns

- **Recent alerts/analysis**: Use `price_observations` (highest resolution)
- **Historical trends**: Use `price_buckets` with appropriate bucket_size
- **Hybrid queries**: Union recent raw data with historical buckets
- **Gap detection**: Compare expected vs actual bucket coverage