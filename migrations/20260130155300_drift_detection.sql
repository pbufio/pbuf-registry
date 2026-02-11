-- +goose Up
-- +goose StatementBegin

-- Add content_hash column to protofiles table for tracking file changes
ALTER TABLE protofiles ADD COLUMN IF NOT EXISTS content_hash VARCHAR(64);

-- Partial index for efficiently finding tags without hashes
CREATE INDEX IF NOT EXISTS idx_protofiles_tag_content_hash_null 
ON protofiles (tag_id) WHERE content_hash IS NULL;

-- Create drift_events table to track detected changes
CREATE TABLE IF NOT EXISTS drift_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    module_id UUID NOT NULL REFERENCES modules(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    event_type VARCHAR(20) NOT NULL CHECK (event_type IN ('added', 'modified', 'deleted')),
    previous_hash VARCHAR(64),
    current_hash VARCHAR(64),
    severity VARCHAR(20) NOT NULL DEFAULT 'info' CHECK (severity IN ('info', 'warning', 'critical')),
    detected_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    acknowledged BOOLEAN NOT NULL DEFAULT FALSE,
    acknowledged_at TIMESTAMP WITH TIME ZONE,
    acknowledged_by VARCHAR(255)
);

-- Create indexes for drift_events
CREATE INDEX IF NOT EXISTS idx_drift_events_module_id ON drift_events (module_id);
CREATE INDEX IF NOT EXISTS idx_drift_events_tag_id ON drift_events (tag_id);
CREATE INDEX IF NOT EXISTS idx_drift_events_detected_at ON drift_events (detected_at);
CREATE INDEX IF NOT EXISTS idx_drift_events_severity ON drift_events (severity);
-- Composite index for common query pattern: unacknowledged events by module
CREATE INDEX IF NOT EXISTS idx_drift_events_module_acknowledged ON drift_events (module_id, acknowledged);

-- Unique constraint for idempotent drift event insertion
-- This prevents duplicate events when the daemon reruns for the same tag pair
CREATE UNIQUE INDEX IF NOT EXISTS idx_drift_events_unique 
ON drift_events (tag_id, filename, event_type, COALESCE(previous_hash, ''), COALESCE(current_hash, ''));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_drift_events_unique;
DROP INDEX IF EXISTS idx_drift_events_module_acknowledged;
DROP INDEX IF EXISTS idx_drift_events_severity;
DROP INDEX IF EXISTS idx_drift_events_detected_at;
DROP INDEX IF EXISTS idx_drift_events_tag_id;
DROP INDEX IF EXISTS idx_drift_events_module_id;
DROP TABLE IF EXISTS drift_events;

DROP INDEX IF EXISTS idx_protofiles_tag_content_hash_null;
ALTER TABLE protofiles DROP COLUMN IF EXISTS content_hash;

-- +goose StatementEnd
