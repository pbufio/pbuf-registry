-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS acl (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    module_name VARCHAR(255) NOT NULL,
    permission VARCHAR(50) NOT NULL CHECK (permission IN ('read', 'write', 'admin')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, module_name)
);

CREATE INDEX IF NOT EXISTS idx_acl_user_id ON acl(user_id);
CREATE INDEX IF NOT EXISTS idx_acl_module_name ON acl(module_name);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_acl_module_name;
DROP INDEX IF EXISTS idx_acl_user_id;
DROP TABLE IF EXISTS acl;
-- +goose StatementEnd
