-- Split price_buckets into resolution-specific tables with retention policies

-- 1. Create new tables for each resolution

-- 5-minute buckets (7 day retention)
CREATE TABLE price_buckets_5m (
    item_id           INTEGER NOT NULL,
    bucket_start      TIMESTAMPTZ NOT NULL,
    avg_high_price    INTEGER,
    high_price_volume BIGINT,
    avg_low_price     INTEGER,
    low_price_volume  BIGINT,
    source            TEXT NOT NULL DEFAULT 'api',
    ingested_at       TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (item_id, bucket_start)
);
SELECT create_hypertable('price_buckets_5m', 'bucket_start');
CREATE INDEX idx_buckets_5m_item ON price_buckets_5m (item_id, bucket_start DESC);

-- 1-hour buckets (1 year retention)
CREATE TABLE price_buckets_1h (
    item_id           INTEGER NOT NULL,
    bucket_start      TIMESTAMPTZ NOT NULL,
    avg_high_price    INTEGER,
    high_price_volume BIGINT,
    avg_low_price     INTEGER,
    low_price_volume  BIGINT,
    source            TEXT NOT NULL DEFAULT 'api',
    ingested_at       TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (item_id, bucket_start)
);
SELECT create_hypertable('price_buckets_1h', 'bucket_start');
CREATE INDEX idx_buckets_1h_item ON price_buckets_1h (item_id, bucket_start DESC);

-- 24-hour buckets (kept forever)
CREATE TABLE price_buckets_24h (
    item_id           INTEGER NOT NULL,
    bucket_start      TIMESTAMPTZ NOT NULL,
    avg_high_price    INTEGER,
    high_price_volume BIGINT,
    avg_low_price     INTEGER,
    low_price_volume  BIGINT,
    source            TEXT NOT NULL DEFAULT 'api',
    ingested_at       TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (item_id, bucket_start)
);
SELECT create_hypertable('price_buckets_24h', 'bucket_start');
CREATE INDEX idx_buckets_24h_item ON price_buckets_24h (item_id, bucket_start DESC);

-- 2. Migrate existing data from price_buckets to new tables
INSERT INTO price_buckets_5m (item_id, bucket_start, avg_high_price, high_price_volume, avg_low_price, low_price_volume, source, ingested_at)
SELECT item_id, bucket_start, avg_high_price, high_price_volume, avg_low_price, low_price_volume, source, ingested_at
FROM price_buckets WHERE bucket_size = '5m'
ON CONFLICT DO NOTHING;

INSERT INTO price_buckets_1h (item_id, bucket_start, avg_high_price, high_price_volume, avg_low_price, low_price_volume, source, ingested_at)
SELECT item_id, bucket_start, avg_high_price, high_price_volume, avg_low_price, low_price_volume, source, ingested_at
FROM price_buckets WHERE bucket_size = '1h'
ON CONFLICT DO NOTHING;

INSERT INTO price_buckets_24h (item_id, bucket_start, avg_high_price, high_price_volume, avg_low_price, low_price_volume, source, ingested_at)
SELECT item_id, bucket_start, avg_high_price, high_price_volume, avg_low_price, low_price_volume, source, ingested_at
FROM price_buckets WHERE bucket_size = '24h'
ON CONFLICT DO NOTHING;

-- 3. Drop old unified table
DROP TABLE price_buckets;

-- 4. Add retention policies (TimescaleDB will auto-prune old chunks)
SELECT add_retention_policy('price_observations', INTERVAL '7 days');
SELECT add_retention_policy('price_buckets_5m', INTERVAL '7 days');
SELECT add_retention_policy('price_buckets_1h', INTERVAL '1 year');
-- price_buckets_24h: no retention policy (keep forever)

-- 5. Optional: Add compression for 24h buckets (compress chunks older than 30 days)
ALTER TABLE price_buckets_24h SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'item_id'
);
SELECT add_compression_policy('price_buckets_24h', INTERVAL '30 days');
