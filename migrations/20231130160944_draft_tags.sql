-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS draft_tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    module_id UUID NOT NULL,
    tag VARCHAR(255) NOT NULL,
    proto_files JSONB NOT NULL,
    dependencies JSONB NOT NULL,
    updated_at timestamp NOT NULL DEFAULT NOW(),
    UNIQUE (module_id, tag)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS draft_tags;
-- +goose StatementEnd
