-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS weight_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    weight_kg REAL NOT NULL,
    log_date DATE NOT NULL, 
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ DEFAULT NULL,

    -- This allows one log per user per day
    CONSTRAINT unique_user_daily_weight UNIQUE(user_id, log_date)
);
CREATE INDEX idx_weight_logs_updated_at ON weight_logs(updated_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_weight_logs_updated_at;
DROP TABLE IF EXISTS weight_logs;
-- +goose StatementEnd
