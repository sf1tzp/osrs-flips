-- Add 5-year retention policy for 24h buckets
-- Previously kept forever; now aligned with Go-side RetentionPolicy["24h"] = 5 years
SELECT add_retention_policy('price_buckets_24h', INTERVAL '5 years');
