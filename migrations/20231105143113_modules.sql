-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS modules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) UNIQUE NOT NULL
    );

CREATE TABLE IF NOT EXISTS tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    module_id UUID NOT NULL,
    tag VARCHAR(255) NOT NULL
    );

CREATE INDEX IF NOT EXISTS idx_modules_name ON modules (name);
CREATE INDEX IF NOT EXISTS idx_tags_module_id_tag ON tags (module_id, tag);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS modules;
-- +goose StatementEnd