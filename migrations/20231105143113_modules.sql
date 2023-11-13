-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS modules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) UNIQUE NOT NULL
    );

CREATE TABLE IF NOT EXISTS tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    module_id UUID NOT NULL,
    tag VARCHAR(255) NOT NULL,
    UNIQUE (module_id, tag)
    );

CREATE TABLE IF NOT EXISTS protofiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tag_id UUID NOT NULL,
    filename TEXT NOT NULL,
    content TEXT NOT NULL,
    UNIQUE (tag_id, filename)
    );

CREATE INDEX IF NOT EXISTS idx_modules_name ON modules (name);
CREATE INDEX IF NOT EXISTS idx_tags_module_id_tag ON tags (module_id, tag);
CREATE INDEX IF NOT EXISTS idx_protofiles_tag_id ON protofiles (tag_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_protofiles_tag_id;
DROP INDEX IF EXISTS idx_tags_module_id_tag;
DROP INDEX IF EXISTS idx_modules_name;

DROP TABLE IF EXISTS protofiles;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS modules;
-- +goose StatementEnd