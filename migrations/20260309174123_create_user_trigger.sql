-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION insertNewUserSettings() RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO user_settings (user_id, units, calories_target, protein_target, carbs_target, fat_target)
    VALUES (NEW.id, 'metric', 2000, 150, 200, 70);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER insert_new_user_settings_trigger
AFTER INSERT ON users
FOR EACH ROW
EXECUTE FUNCTION insertNewUserSettings();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS insert_new_user_settings_trigger ON users;
DROP FUNCTION IF EXISTS insertNewUserSettings();
-- +goose StatementEnd
