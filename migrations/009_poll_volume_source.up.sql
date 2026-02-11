-- Track whether volume polling was enabled manually or automatically by SignalComputer
ALTER TABLE items ADD COLUMN poll_volume_source TEXT NOT NULL DEFAULT 'manual';
ALTER TABLE items ADD COLUMN poll_volume_auto_at TIMESTAMPTZ;
