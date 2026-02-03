-- Pre-aggregated bucket data from /timeseries and /5m API endpoints
-- Stores 5m, 1h, 24h aggregates with volume data

CREATE TABLE price_buckets (
    item_id           INTEGER NOT NULL,
    bucket_start      TIMESTAMPTZ NOT NULL,
    bucket_size       TEXT NOT NULL,  -- '5m', '1h', '24h'
    avg_high_price    INTEGER,
    high_price_volume BIGINT,
    avg_low_price     INTEGER,
    low_price_volume  BIGINT,
    source            TEXT NOT NULL DEFAULT 'api',  -- 'api' or 'computed'
    ingested_at       TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (item_id, bucket_start, bucket_size)
);

-- Convert to TimescaleDB hypertable
SELECT create_hypertable('price_buckets', 'bucket_start');

-- Index for item lookups with time ordering (historical trend queries)
CREATE INDEX idx_buckets_item_time ON price_buckets (item_id, bucket_start DESC);

-- Index for filtering by bucket size (e.g., get all 1h buckets for an item)
CREATE INDEX idx_buckets_item_size ON price_buckets (item_id, bucket_size, bucket_start DESC);
