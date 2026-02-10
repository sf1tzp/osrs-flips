CREATE TABLE IF NOT EXISTS sync_no_data (
    bucket_size TEXT NOT NULL,
    bucket_start TIMESTAMPTZ NOT NULL,
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bucket_size, bucket_start)
);
