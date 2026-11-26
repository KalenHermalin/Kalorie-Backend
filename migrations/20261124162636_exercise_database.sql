-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS exercises (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    equipment TEXT NOT NULL,
    primary_muscle TEXT NOT NULL,
    secondary_muscles TEXT[] NOT NULL DEFAULT '{}',
    steps TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_exercises_name_lower ON exercises (lower(name));
CREATE INDEX idx_exercises_name_trgm ON exercises USING GIN (lower(name) gin_trgm_ops);

CREATE TABLE IF NOT EXISTS exercise_aliases (
    alias TEXT PRIMARY KEY CHECK (
        alias = lower(btrim(alias))
    ),
    exercise_id TEXT NOT NULL REFERENCES exercises(id) ON DELETE CASCADE
);
CREATE INDEX idx_exercise_aliases_exercise_id ON exercise_aliases(exercise_id);
CREATE INDEX idx_exercise_aliases_alias_trgm ON exercise_aliases USING GIN (alias gin_trgm_ops);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS exercise_aliases;
DROP TABLE IF EXISTS exercises;


-- +goose StatementEnd
