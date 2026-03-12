package store

import (
	"context"
	"database/sql"
	_ "github.com/lib/pq" // Your Postgres driver
	"github.com/stretchr/testify/assert"
	"log"
	"main/internal/auth"
	"main/internal/models"
	"testing"
	"time"
)

var testDB *sql.DB

func CheckDBConnection(db *sql.DB) {
	connected := false
	for range 10 {
		err := db.Ping()
		if err == nil {
			connected = true
			break
		}
		log.Println("Waiting For Database Conneciton...")
		time.Sleep(3 * time.Second)
	}
	if !connected {
		log.Fatalln("Could not connect to database after 30 seconds. Exiting.")
	}
}

func TestDeleteRefreshToken(t *testing.T) {
	store := NewPostgressUserStore(testDB) // Pass the transaction as the DB
	clearTables(t)
	defer clearTables(t)
	payload := &models.AuthPayload{
		Email:    "kalen@laurier.ca",
		ID:       "12345",
		Provider: "github",
	}
	store.WithTx(context.Background(), func(tx *sql.Tx) error {
		// Test Case 1: First Login (Insert)
		user, err := store.UpsertUserWithAuth(context.Background(), tx, payload)
		if err != nil {
			t.Fatalf("Failed to insert new user: %v", err)
		}

		if user.Email != payload.Email {
			t.Errorf("Expected email %s, got %s", payload.Email, user.Email)
		}
		refresh, err := auth.GenerateRefreshToken(user.ID, time.Now().Add(time.Hour*24*30), "test")
		if err != nil {
			return err
		}
		err = store.SaveRefreshToken(context.Background(), tx, refresh, user.ID, time.Now().Add(time.Hour*24*30))
		if err != nil {
			return err
		}

		err = store.DeleteRefreshToken(context.Background(), tx, refresh, user.ID)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		err = store.DeleteRefreshToken(context.Background(), tx, refresh, user.ID)

		// This SHOULD return an error now because the rows affected will be 0
		if err == nil {
			t.Error("Expected an error when deleting a non-existent token, but got nil")
		}
		return nil
	})

}

func TestRefreshRefreshToken(t *testing.T) {
	store := NewPostgressUserStore(testDB) // Pass the transaction as the DB
	clearTables(t)
	defer clearTables(t)
	payload := &models.AuthPayload{
		Email:    "kalen@laurier.ca",
		ID:       "12345",
		Provider: "github",
	}
	store.WithTx(context.Background(), func(tx *sql.Tx) error {
		// Test Case 1: First Login (Insert)
		user, err := store.UpsertUserWithAuth(context.Background(), tx, payload)
		if err != nil {
			t.Fatalf("Failed to insert new user: %v", err)
		}

		if user.Email != payload.Email {
			t.Errorf("Expected email %s, got %s", payload.Email, user.Email)
		}
		refresh, err := auth.GenerateRefreshToken(user.ID, time.Now().Add(time.Hour*24*30), "test")
		if err != nil {
			return err
		}
		err = store.SaveRefreshToken(context.Background(), tx, refresh, user.ID, time.Now().Add(time.Hour*24*30))
		if err != nil {
			return err
		}

		user, err = store.FindRefreshToken(context.Background(), tx, refresh, user.ID)
		if err != nil {
			t.Errorf("Should have found token: %v", err)
		}

		err = store.DeleteRefreshToken(context.Background(), tx, refresh, user.ID)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		_, err = store.FindRefreshToken(context.Background(), tx, refresh, user.ID)
		if err == nil {
			t.Error("Expected an error: token should not be findable after deleting")
		}

		refreshToken, err := auth.GenerateRefreshToken(user.ID, time.Now().Add(time.Hour*24*30), "test")
		if err != nil {
			return err
		}
		err = store.SaveRefreshToken(context.Background(), tx, refreshToken, user.ID, time.Now().Add(time.Hour*24*30))
		if err != nil {
			return err
		}
		_, err = store.FindRefreshToken(context.Background(), tx, refreshToken, user.ID)
		if err != nil {
			t.Errorf("Error, refresh rotation failed to save")
		}
		return nil
	})

}
func TestUpdateUserSettingsSuccess(t *testing.T) {

	store := NewPostgressUserStore(testDB) // Pass the transaction as the DB
	clearTables(t)
	defer clearTables(t)
	payload := &models.AuthPayload{
		Email:    "kalen@laurier.ca",
		ID:       "12345",
		Provider: "github",
	}

	store.WithTx(context.Background(), func(tx *sql.Tx) error {

		user, err := store.UpsertUserWithAuth(context.Background(), tx, payload)
		if err != nil {
			t.Fatalf("Failed to insert new user: %v", err)
		}

		if user.Email != payload.Email {
			t.Errorf("Expected email %s, got %s", payload.Email, user.Email)
		}
		now := time.Now().UTC().Truncate(time.Microsecond) // Strip nanoseconds and monotonic clock
		settings := &models.UserSettings{
			CaloriesTarget: 3000,
			ProteinTarget:  150,
			FatTarget:      25,
			CarbsTarget:    400,
			Units:          "metric",
			Theme:          "dark",
			UpdatedAt:      now,
			DeletedAt:      sql.NullTime{Valid: false},
		}
		err = store.UpdateUserSettings(context.Background(), tx, user.ID, settings)
		if err != nil {
			t.Errorf("%s", "Expected success but got:"+err.Error())
		}

		newSettings, err := store.GetUserSettings(context.Background(), tx, user.ID)
		if err != nil {

			t.Errorf("%s", "Error retreiving new settings:"+err.Error())
			return err
		}

		AssertSettingsEqual(t, settings, newSettings)
		return nil
	})
}
func TestUpdateUserSettingsFailure(t *testing.T) {

	store := NewPostgressUserStore(testDB) // Pass the transaction as the DB
	clearTables(t)
	defer clearTables(t)
	payload := &models.AuthPayload{
		Email:    "kalen@laurier.ca",
		ID:       "12345",
		Provider: "github",
	}

	store.WithTx(context.Background(), func(tx *sql.Tx) error {

		user, err := store.UpsertUserWithAuth(context.Background(), tx, payload)
		if err != nil {
			t.Fatalf("Failed to insert new user: %v", err)
		}

		if user.Email != payload.Email {
			t.Errorf("Expected email %s, got %s", payload.Email, user.Email)
		}
		settings := &models.UserSettings{
			CaloriesTarget: 3000,
			ProteinTarget:  150,
			FatTarget:      25,
			CarbsTarget:    400,
			Units:          "metric",
			Theme:          "dark",
			UpdatedAt:      time.Now().Add(time.Minute * 10),
			DeletedAt:      sql.NullTime{Valid: false},
		}
		err = store.UpdateUserSettings(context.Background(), tx, "b38e5cee-8726-47cb-a4b6-755f8e521818", settings)
		if err == nil {
			t.Errorf("%s", "Expected to get an error because userID: "+user.ID+"abcd"+"does not exist")
		}
		t.Log("Successfully failed with error: " + err.Error())
		return nil
	})
}
func AssertSettingsEqual(t *testing.T, expected, actual *models.UserSettings) {
	t.Helper()

	// 1. Compare standard fields
	assert.Equal(t, expected.Units, actual.Units)
	assert.Equal(t, expected.CaloriesTarget, actual.CaloriesTarget)
	assert.Equal(t, expected.ProteinTarget, actual.ProteinTarget)
	assert.Equal(t, expected.CarbsTarget, actual.CarbsTarget)
	assert.Equal(t, expected.FatTarget, actual.FatTarget)
	assert.Equal(t, expected.Theme, actual.Theme)

	// 2. Compare UpdatedAt (Truncate to Microsecond for DB precision)
	assert.True(t, expected.UpdatedAt.Truncate(time.Microsecond).Equal(actual.UpdatedAt.Truncate(time.Microsecond)),
		"UpdatedAt mismatch: expected %v, got %v", expected.UpdatedAt, actual.UpdatedAt)

	// 3. Compare DeletedAt (sql.NullTime)
	assert.Equal(t, expected.DeletedAt.Valid, actual.DeletedAt.Valid, "DeletedAt 'Valid' state mismatch")

	if expected.DeletedAt.Valid {
		// Only compare the time values if both are actually set (not NULL)
		assert.True(t, expected.DeletedAt.Time.Truncate(time.Microsecond).Equal(actual.DeletedAt.Time.Truncate(time.Microsecond)),
			"DeletedAt Time mismatch: expected %v, got %v", expected.DeletedAt.Time, actual.DeletedAt.Time)
	}
}
func TestUpsertUserWithAuth(t *testing.T) {
	// Start a transaction to keep the test "atomic"
	store := NewPostgressUserStore(testDB) // Pass the transaction as the DB
	clearTables(t)
	defer clearTables(t)
	payload := &models.AuthPayload{
		Email:    "kalen@laurier.ca",
		ID:       "12345",
		Provider: "github",
	}

	store.WithTx(context.Background(), func(tx *sql.Tx) error {

		user, err := store.UpsertUserWithAuth(context.Background(), tx, payload)
		if err != nil {
			t.Fatalf("Failed to insert new user: %v", err)
		}

		if user.Email != payload.Email {
			t.Errorf("Expected email %s, got %s", payload.Email, user.Email)
		}

		// Test Case 2: Second Login (Update)
		// This proves your "ON CONFLICT" logic works!
		user2, err := store.UpsertUserWithAuth(context.Background(), tx, payload)
		if err != nil {
			t.Fatalf("Failed to upsert existing user: %v", err)
		}

		if user.ID != user2.ID {
			t.Error("Expected same user ID for duplicate login, but got a new one")
		}
		return nil
	})

}

func TestUpsertUserWeightLog(t *testing.T) {
	// Start a transaction to keep the test "atomic"
	store := NewPostgressUserStore(testDB) // Pass the transaction as the DB
	clearTables(t)
	defer clearTables(t)
	payload := &models.AuthPayload{
		Email:    "kalen@laurier.ca",
		ID:       "12345",
		Provider: "github",
	}

	store.WithTx(context.Background(), func(tx *sql.Tx) error {

		user, err := store.UpsertUserWithAuth(context.Background(), tx, payload)
		if err != nil {
			t.Fatalf("Failed to insert new user: %v", err)
		}

		if user.Email != payload.Email {

			t.Errorf("Expected email %s, got %s", payload.Email, user.Email)
		}
		weightLog := &models.WeightLog{
			ID:        "b38e5cee-8726-47cb-a4b6-755f8e521818",
			WeightKg:  55,
			LogDate:   time.Now(),
			UpdatedAt: time.Now(),
			DeletedAt: sql.NullTime{Valid: false},
		}

		// Test first weight log
		err = store.UpsertUserWeightLog(context.Background(), tx, user.ID, weightLog)
		if err != nil {
			t.Fatalf("Failed to insert new user weight log: %v", err)
		}

		// Test updating weight log

		weightLog2 := &models.WeightLog{
			ID:        "b38e5cee-8726-47cb-a4b6-755f8e521818",
			WeightKg:  45,
			LogDate:   time.Date(weightLog.LogDate.Year(), weightLog.LogDate.Month(), weightLog.LogDate.Day(), 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Now().Add(time.Minute * 10),
			DeletedAt: sql.NullTime{Valid: false},
		}
		err = store.UpsertUserWeightLog(context.Background(), tx, user.ID, weightLog2)
		if err != nil {
			t.Fatalf("Failed to upsert (update) users weight log: %v", err)
		}

		log, err := store.GetUserWeightLogById(context.Background(), tx, user.ID, weightLog.ID)
		if err != nil {
			t.Fatalf("Failed to retreive user weight log by id: %v", err)
		}
		AssertWeightLogEqual(t, weightLog2, log)
		return nil

	})

}
func AssertWeightLogEqual(t *testing.T, expected, actual *models.WeightLog) {
	t.Helper()

	// ... (other fields)

	// Using assert.Equal here will force the test to print the values on failure
	expectedDate := expected.LogDate.Truncate(time.Microsecond).UTC()
	actualDate := actual.LogDate.Truncate(time.Microsecond).UTC()

	assert.Equal(t, expectedDate, actualDate, "LogDate mismatch (truncated to microsecond)")

	expectedUpdate := expected.UpdatedAt.Truncate(time.Microsecond).UTC()
	actualUpdate := actual.UpdatedAt.Truncate(time.Microsecond).UTC()

	assert.Equal(t, expectedUpdate, actualUpdate, "UpdatedAt mismatch")

	// ... (rest of the helper)
}
func clearTables(t *testing.T) {
	// RESTART IDENTITY resets your auto-incrementing IDs back to 1
	// CASCADE handles the foreign key dependencies automatically
	query := `TRUNCATE users, provider_identities, refresh_tokens RESTART IDENTITY CASCADE`
	_, err := testDB.Exec(query)
	if err != nil {
		t.Fatalf("Failed to clear test database: %v", err)
	}
}
