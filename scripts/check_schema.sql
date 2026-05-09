-- Check tables exist
SELECT tablename FROM pg_tables WHERE schemaname = 'public';

-- Check hypertables
SELECT hypertable_name, num_dimensions FROM timescaledb_information.hypertables;

-- Check indexes on price_observations
SELECT indexname FROM pg_indexes WHERE tablename = 'price_observations';

-- Check indexes on price_buckets
SELECT indexname FROM pg_indexes WHERE tablename = 'price_buckets';
