-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN IF NOT EXISTS token_lookup VARCHAR(64);
CREATE INDEX IF NOT EXISTS idx_users_token_lookup ON users(token_lookup);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_users_token_lookup;
ALTER TABLE users DROP COLUMN IF EXISTS token_lookup;
-- +goose StatementEnd
