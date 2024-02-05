-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS tokens (
    "token" VARCHAR(100) PRIMARY KEY NOT NULL DEFAULT '',
    "name" VARCHAR(255) NOT NULL DEFAULT '',
    "role" VARCHAR(100) NOT NULL,
    "parent_token" VARCHAR(100) NOT NULL DEFAULT '',
    "is_deleted" boolean NOT NULL DEFAULT '0',
    "created_at" TIMESTAMP NOT NULL DEFAULT NOW(),
    "updated_at" TIMESTAMP NOT NULL DEFAULT NOW(),
    "expires_at" TIMESTAMP NOT NULL DEFAULT NOW()
    );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tokens;
-- +goose StatementEnd
