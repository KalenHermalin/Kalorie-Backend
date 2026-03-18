-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS food_logs (
  id UUID PRIMARY KEY NOT NULL,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  time TIMESTAMPTZ NOT NULL,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  deleted_at TIMESTAMPTZ DEFAULT NULL

);

CREATE INDEX idx_log_entries_date ON food_logs(created_at);
CREATE INDEX idx_log_entries_updated_at ON food_logs(updated_at);


CREATE TABLE IF NOT EXISTS food_log_items (
  id UUID PRIMARY KEY NOT NULL,
  log_id UUID NOT NULL,
  food_id TEXT,
  food_name TEXT NOT NULL,
  serving_id TEXT,
  quantity REAL,
  unit TEXT,
  cal INTEGER NOT NULL,
  fat REAL NOT NULL,
  carbs REAL NOT NULL,
  protein REAL NOT NULL,
  FOREIGN KEY(log_id) REFERENCES food_logs(id) ON DELETE CASCADE ON UPDATE NO ACTION,
  updated_at TIMESTAMPTZ NOT NULL,
  deleted_at TIMESTAMPTZ DEFAULT NULL

);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS food_logs;
DROP TABLE IF EXISTS food_log_items;
-- +goose StatementEnd
