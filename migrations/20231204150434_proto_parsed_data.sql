-- +goose Up
-- +goose StatementBegin
ALTER TABLE tags ADD COLUMN is_processed BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS proto_parsed_data (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tag_id UUID NOT NULL,
    filename TEXT NOT NULL,
    json JSONB NOT NULL,
    UNIQUE (tag_id, filename)
    );

CREATE TABLE IF NOT EXISTS tag_meta (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tag_id UUID NOT NULL,
    meta JSONB NOT NULL,
    UNIQUE (tag_id)
    );

CREATE INDEX IF NOT EXISTS idx_tags_is_processed ON tags (is_processed);
CREATE INDEX IF NOT EXISTS idx_proto_parsed_data_tag_id ON proto_parsed_data (tag_id);
CREATE INDEX IF NOT EXISTS idx_tag_meta_tag_id ON tag_meta (tag_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_tags_is_processed;
ALTER TABLE tags DROP COLUMN is_processed;

DROP INDEX IF EXISTS idx_proto_parsed_data_tag_id;
DROP INDEX IF EXISTS idx_tag_meta_tag_id;

DROP TABLE IF EXISTS proto_parsed_data;
DROP TABLE IF EXISTS tag_meta;
-- +goose StatementEnd
