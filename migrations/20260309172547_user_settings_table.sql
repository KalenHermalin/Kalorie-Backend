-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_settings (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    units TEXT NOT NULL,
    calories_target INTEGER NOT NULL,
    protein_target INTEGER NOT NULL,
    carbs_target INTEGER NOT NULL,
    fat_target INTEGER NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ DEFAULT NULL,
    theme TEXT NOT NULL DEFAULT 'system', 
    CONSTRAINT check_allowed_themes 
        CHECK (theme IN ('system', 'light', 'dark'))
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_settings;
-- +goose StatementEnd
