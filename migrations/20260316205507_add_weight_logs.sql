-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS exercise_logs (
    id UUID PRIMARY KEY NOT NULL, 
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exercise_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL
);

CREATE INDEX idx_exercise_logs_updated_at ON exercise_logs(updated_at);
CREATE INDEX idx_exercise_logs_created_at ON exercise_logs(created_at);
CREATE INDEX idx_exercise_logs_user_id ON exercise_logs(user_id);


CREATE TABLE IF NOT EXISTS exercise_sets (
    id UUID PRIMARY KEY NOT NULL,
    log_id UUID NOT NULL REFERENCES exercise_logs (id) ON DELETE CASCADE,
    set_number INTEGER NOT NULL,
    weight REAL NOT NULL,
    reps INTEGER NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL, 
    deleted_at TIMESTAMPTZ DEFAULT NULL
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS exercise_sets;
DROP TABLE IF EXISTS exercise_logs;
-- +goose StatementEnd
