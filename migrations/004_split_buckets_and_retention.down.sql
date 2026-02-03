-- Reverse the bucket split and retention policies
-- WARNING: This will lose the bucket_size column - data migration may not be perfect

-- 1. Remove retention and compression policies
SELECT remove_retention_policy('price_observations', if_exists => true);
SELECT remove_retention_policy('price_buckets_5m', if_exists => true);
SELECT remove_retention_policy('price_buckets_1h', if_exists => true);
SELECT remove_compression_policy('price_buckets_24h', if_exists => true);

-- 2. Recreate unified price_buckets table
CREATE TABLE price_buckets (
    item_id           INTEGER NOT NULL,
    bucket_start      TIMESTAMPTZ NOT NULL,
    bucket_size       TEXT NOT NULL,
    avg_high_price    INTEGER,
    high_price_volume BIGINT,
    avg_low_price     INTEGER,
    low_price_volume  BIGINT,
    source            TEXT NOT NULL DEFAULT 'api',
    ingested_at       TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (item_id, bucket_start, bucket_size)
);
SELECT create_hypertable('price_buckets', 'bucket_start');
CREATE INDEX idx_buckets_item_time ON price_buckets (item_id, bucket_start DESC);
CREATE INDEX idx_buckets_item_size ON price_buckets (item_id, bucket_size, bucket_start DESC);

-- 3. Migrate data back (add bucket_size column)
INSERT INTO price_buckets (item_id, bucket_start, bucket_size, avg_high_price, high_price_volume, avg_low_price, low_price_volume, source, ingested_at)
SELECT item_id, bucket_start, '5m', avg_high_price, high_price_volume, avg_low_price, low_price_volume, source, ingested_at
FROM price_buckets_5m
ON CONFLICT DO NOTHING;

INSERT INTO price_buckets (item_id, bucket_start, bucket_size, avg_high_price, high_price_volume, avg_low_price, low_price_volume, source, ingested_at)
SELECT item_id, bucket_start, '1h', avg_high_price, high_price_volume, avg_low_price, low_price_volume, source, ingested_at
FROM price_buckets_1h
ON CONFLICT DO NOTHING;

INSERT INTO price_buckets (item_id, bucket_start, bucket_size, avg_high_price, high_price_volume, avg_low_price, low_price_volume, source, ingested_at)
SELECT item_id, bucket_start, '24h', avg_high_price, high_price_volume, avg_low_price, low_price_volume, source, ingested_at
FROM price_buckets_24h
ON CONFLICT DO NOTHING;

-- 4. Drop resolution-specific tables
DROP TABLE price_buckets_5m;
DROP TABLE price_buckets_1h;
DROP TABLE price_buckets_24h;
