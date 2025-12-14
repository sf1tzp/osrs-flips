# Database considerations and principles

## Core Principles for Multi-Writer Time-Series Systems

### 1. **Idempotency is Your Best Friend**

Every write operation should be safely repeatable. If a writer crashes and restarts, or if two writers somehow process the same data, the database should end up in the same correct state.

**Implications:**
- Use `ON CONFLICT` clauses extensively (upsert patterns)
- Design primary keys carefully - they define your idempotency boundaries
- For `price_observations`: `(item_id, observed_at)` means "same item at same timestamp = same observation"
- For `price_buckets`: `(item_id, bucket_start, bucket_size)` means "same item, bucket, and granularity = same aggregate"

**Gotcha:** Be careful with partial updates. If you `UPDATE` without `WHERE` conditions that match your PK, you might corrupt data.

### 2. **Timestamps Are Tricky - Pick a Single Source of Truth**

We have multiple timestamp concepts floating around:
- `observed_at` - when WE polled the API
- `high_time`/`low_time` - when the OSRS game server last saw that price
- `bucket_start` - which time bucket this data belongs to
- `ingested_at` - when we wrote to our DB

**Principle: Use the API's timestamps as canonical, not our system clock**

Why? If your poller has clock skew, or takes 2 seconds to process a response, you want to honor when the *price actually changed*, not when you happened to read it.

**Gotcha:** Different systems might be in different timezones. Store everything in UTC, convert only at display time.

### 3. **Partition Boundaries and Writer Coordination**

With time-based partitioning, you need to think about:
- **Who creates new partitions?** If all three writers try to INSERT into a non-existent partition simultaneously, you'll get race conditions
- **Partition maintenance:** Old partitions need to be dropped on schedule

**Solutions:**
- Pre-create partitions ahead of time (cron job or TimescaleDB's automatic partitioning)
- Or: Have a single "partition manager" service
- Or: Use TimescaleDB's built-in chunk management (recommended)

**Gotcha:** If you manually manage partitions, a writer can fail with "no partition exists" errors during rollover windows.

### 4. **Gap Detection Must Be Precise**

Gaps are subtle because you need to distinguish:
- **Never existed:** No data was ever collected
- **Deleted:** Data existed but was lifecycle'd out
- **Expected missing:** API returned empty/null for this item in this bucket
- **Actual gap:** We should have data but don't

**Principle: Track metadata about what you've attempted**

Consider a `ingestion_log` table:
```sql
ingestion_attempts (
    source_type,  -- 'latest_poll', 'timeseries_5m', etc.
    time_range_start,
    time_range_end,
    attempted_at,
    status,  -- 'success', 'partial', 'failed'
    items_processed
)
```

This lets gap-filler know: "Did we try to fetch 5m buckets for 2:00-2:05? If yes and nothing's in price_buckets, the API had no data. If no, we need to fetch."

**Gotcha:** Without this, your gap-filler might infinitely retry fetching data that doesn't exist, or miss real gaps.

### 5. **Handle Concurrent Writes with Explicit Locking Strategy**

Three writers might try to write to the same row simultaneously. Options:

**Option A: Optimistic - Last Write Wins (LWW)**
```sql
INSERT INTO price_buckets (...)
VALUES (...)
ON CONFLICT (item_id, bucket_start, bucket_size)
DO UPDATE SET
    avg_high_price = EXCLUDED.avg_high_price,
    ingested_at = EXCLUDED.ingested_at
WHERE price_buckets.ingested_at < EXCLUDED.ingested_at;  -- Only if newer
```

**Option B: Pessimistic - Advisory Locks**
```sql
SELECT pg_advisory_xact_lock(hashtext(item_id || bucket_start));
-- Now we hold exclusive lock for this bucket until transaction commits
```

**Option C: Source Priority - Prefer API over Computed**
```sql
ON CONFLICT (...) DO UPDATE SET ...
WHERE price_buckets.source = 'computed' AND EXCLUDED.source = 'api';
```

**My recommendation:** Use Option C. API data (with volume) is higher quality than our computed aggregates.

**Gotcha:** If backfill and gap-filler both run, you want backfill's API data to win over gap-filler's potentially computed data.

### 6. **Lifecycle Operations Need Isolation**

When pruning old `price_observations`, you don't want:
- A query to fail mid-read because rows disappeared
- The poller to insert into a partition you're about to drop

**Principle: Use partition-level operations, not row-level DELETEs**

TimescaleDB's `drop_chunks()` is atomic and fast:
```sql
SELECT drop_chunks('price_observations', INTERVAL '30 days');
```

Much better than:
```sql
DELETE FROM price_observations WHERE observed_at < NOW() - INTERVAL '30 days';
-- This locks rows, bloats tables, and is slow
```

**Gotcha:** If you lifecycle during heavy write periods, you might cause lock contention. Run during low-traffic windows.

### 7. **Rate Limiting State Needs to Be Shared**

If multiple instances of `ingest-backfill` run, they need to coordinate rate limiting. Options:

**Option A: Redis/Memcached rate limit counter**
**Option B: Database-backed rate limit tracking**
**Option C: Single-instance backfiller with internal rate limit**

For a hobby project, Option C is simplest - only run one backfiller instance.

**Gotcha:** If you horizontally scale backfillers without coordination, you'll hammer the API.

### 8. **Transaction Boundaries Match Work Units**

For `ingest-poller` reading `/latest`:
- One API call returns ~4000 items
- **Transaction scope:** All 4000 inserts in one transaction
- Why? Atomic snapshot - either we have the full poll result or we retry

For `ingest-backfill` reading `/timeseries`:
- One API call returns 365 buckets for ONE item
- **Transaction scope:** All 365 buckets for that item in one transaction
- Why? If we crash mid-item, we can resume from the next item

**Gotcha:** Don't make transactions too large (memory) or too small (overhead/consistency).

---

## Potential Gotchas Checklist

1. **Clock skew between services** → Use API timestamps as source of truth
2. **Partition doesn't exist** → Pre-create or use TimescaleDB auto-chunking
3. **Infinite gap-filling loops** → Track ingestion attempts explicitly
4. **Concurrent bucket updates** → Use ON CONFLICT with source priority
5. **Slow DELETEs during lifecycle** → Use partition dropping instead
6. **Rate limit exceeded** → Coordinate between backfillers, or run singleton
7. **Large transaction failure** → Chunk writes appropriately (but maintain atomicity for logical units)
8. **NULL vs 0 vs missing** → Be explicit about what each means in your data model
9. **Duplicate polling** → Ensure `observed_at` is collision-resistant (microsecond precision?)
10. **Timezone confusion** → Everything in UTC, always

---

**Does this framework resonate with you?** Should we drill into any of these areas, or would you like to start designing the actual database schema with these principles in mind?