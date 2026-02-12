-- +goose Up
-- +goose StatementBegin
CREATE TABLE provider_identities (
id SERIAL PRIMARY KEY,
user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
provider TEXT NOT NULL,
CONSTRAINT check_allowed_providers 
    CHECK (provider IN ('github', 'google', 'apple')),
provider_user_id TEXT NOT NULL,
UNIQUE(provider, provider_user_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE provider_identities;
-- +goose StatementEnd
