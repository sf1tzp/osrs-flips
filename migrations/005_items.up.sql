-- Item metadata table for enriching price data and selective volume polling
-- Populated from OSRS Wiki /mapping endpoint

CREATE TABLE items (
    item_id     INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    examine     TEXT,
    members     BOOLEAN DEFAULT FALSE,
    buy_limit   INTEGER,
    high_alch   INTEGER,
    low_alch    INTEGER,
    ge_value    INTEGER,
    icon        TEXT,

    -- Volume polling configuration
    -- When true, VolumePoller will fetch 5m timeseries for this item
    poll_volume BOOLEAN DEFAULT FALSE,

    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Partial index for efficient lookup of items to poll
CREATE INDEX idx_items_poll_volume ON items(item_id) WHERE poll_volume = TRUE;

-- Index for name searches
CREATE INDEX idx_items_name ON items(name);

-- Index for buy limit filtering (common query pattern)
CREATE INDEX idx_items_buy_limit ON items(buy_limit) WHERE buy_limit IS NOT NULL;
