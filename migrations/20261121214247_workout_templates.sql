-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS workout_templates (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    template_name TEXT NOT NULL,
    exercises JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ DEFAULT NULL
);
CREATE INDEX idx_exercise_logs_user_id_exercise_id ON exercise_logs(user_id, exercise_id, created_at DESC);
CREATE INDEX idx_workout_templates_user_id_update_at ON workout_templates(user_id, updated_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS workout_templates;
DROP INDEX IF EXISTS idx_exercise_logs_user_id_exercise_id;
DROP INDEX IF EXISTS idx_workout_templates_user_id_update_at;
-- +goose StatementEnd
