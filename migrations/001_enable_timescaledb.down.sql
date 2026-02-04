-- Note: Dropping timescaledb extension will fail if hypertables exist
-- This is intentional - you must drop tables first
DROP EXTENSION IF EXISTS timescaledb;
