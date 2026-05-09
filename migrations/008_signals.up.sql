-- Signals table for trading signal detection (spread widening, RSI, MACD, etc.)
-- Regular table (not hypertable) — low volume, one active signal per type per item.
CREATE TABLE signals (
    id          BIGSERIAL PRIMARY KEY,
    item_id     INTEGER NOT NULL REFERENCES items(item_id),
    signal_type TEXT NOT NULL,
    score       DOUBLE PRECISION NOT NULL,
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ NOT NULL,
    UNIQUE (item_id, signal_type)
);

CREATE INDEX idx_signals_active ON signals (signal_type, expires_at);
CREATE INDEX idx_signals_item ON signals (item_id);
