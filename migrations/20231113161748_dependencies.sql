-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS dependencies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tag_id UUID NOT NULL,
    dependency_tag_id UUID NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS dependencies_tag_id_dependency_tag_id_idx ON dependencies (tag_id, dependency_tag_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS dependencies_tag_id_dependency_tag_id_idx;
DROP TABLE IF EXISTS dependencies;
-- +goose StatementEnd
