-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_dependencies_dependency_tag_id ON dependencies (dependency_tag_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_dependencies_dependency_tag_id;
-- +goose StatementEnd
