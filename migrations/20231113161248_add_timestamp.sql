-- +goose Up
-- +goose StatementBegin
ALTER TABLE modules
ADD COLUMN updated_at timestamp NOT NULL DEFAULT NOW();

ALTER TABLE tags
ADD COLUMN updated_at timestamp NOT NULL DEFAULT NOW();

ALTER TABLE protofiles
ADD COLUMN updated_at timestamp NOT NULL DEFAULT NOW();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE protofiles
DROP COLUMN updated_at;

ALTER TABLE tags
DROP COLUMN updated_at;

ALTER TABLE modules
DROP COLUMN updated_at;
-- +goose StatementEnd
