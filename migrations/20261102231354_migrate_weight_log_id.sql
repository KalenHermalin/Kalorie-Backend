-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
update weight_logs
SET id = uuid_generate_v5 (
    'f2de908e-4ceb-4099-b4b5-a450e7e5544a', -- UUID Namespace,
    concat(to_char(log_date, 'YYYY-MM-DD'), '-', user_id)
)

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- ONE WAY MIGRATION
SELECT 'down SQL query';
-- +goose StatementEnd
