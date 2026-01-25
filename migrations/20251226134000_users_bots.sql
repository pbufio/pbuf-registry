-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    token VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('user', 'bot')),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_token ON users(token);
CREATE INDEX IF NOT EXISTS idx_users_type ON users(type);
CREATE INDEX IF NOT EXISTS idx_users_name ON users(name);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_users_name;
DROP INDEX IF EXISTS idx_users_type;
DROP INDEX IF EXISTS idx_users_token;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
