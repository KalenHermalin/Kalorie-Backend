-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS remind_user (
id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
email TEXT UNIQUE NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS remind_user;
-- +goose StatementEnd
