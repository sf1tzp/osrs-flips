-- Price observations from /latest API polling
-- High-resolution raw data (30-60s intervals)

CREATE TABLE price_observations (
    item_id        INTEGER NOT NULL,
    observed_at    TIMESTAMPTZ NOT NULL,
    high_price     INTEGER,
    high_time      TIMESTAMPTZ,
    low_price      INTEGER,
    low_time       TIMESTAMPTZ,
    ingested_at    TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (item_id, observed_at)
);

-- Convert to TimescaleDB hypertable
-- Chunks by observed_at, default chunk interval (7 days)
SELECT create_hypertable('price_observations', 'observed_at');

-- Index for item lookups with time ordering (technical analysis queries)
CREATE INDEX idx_observations_item_time ON price_observations (item_id, observed_at DESC);

-- Index for price threshold alerting
CREATE INDEX idx_observations_item_prices ON price_observations (item_id, high_price, low_price);
