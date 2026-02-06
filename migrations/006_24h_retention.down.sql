-- Remove the 5-year retention policy for 24h buckets (revert to keeping forever)
SELECT remove_retention_policy('price_buckets_24h', if_exists => true);
