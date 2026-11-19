package store

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"main/internal/models"
	"testing"
	"time"

	_ "github.com/lib/pq" // Your Postgres driver
	"github.com/stretchr/testify/assert"
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

func TestUpdateUserSettingsSuccess(t *testing.T) {

	userStore := NewPostgressUserStore(testDB) // Pass the transaction as the DB
	authStore := NewPostgressAuthStore(testDB) // Pass the transaction as the DB
	clearTables(t)
	defer clearTables(t)
	email := "kalen@laurier.ca"
	payload := &models.AuthPayload{
		Email: &email,
		ID:    "12345",
	}

	userStore.WithTx(context.Background(), func(tx *sql.Tx) error {

		user, err := authStore.UpsertUserWithAuth(context.Background(), tx, payload)
		if err != nil {
			t.Fatalf("Failed to insert new user: %v", err)
		}

		if payload.Email != nil && user.Email != nil && *user.Email != *payload.Email {
			t.Errorf("Expected email %s, got %s", *payload.Email, *user.Email)
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
			DeletedAt:      nil,
		}
		err = userStore.UpdateUserSettings(context.Background(), tx, user.ID, settings)
		if err != nil {
			t.Errorf("%s", "Expected success but got:"+err.Error())
		}

		newSettings, err := userStore.GetUserSettings(context.Background(), tx, user.ID)
		if err != nil {

			t.Errorf("%s", "Error retreiving new settings:"+err.Error())
			return err
		}

		AssertSettingsEqual(t, settings, newSettings)
		return nil
	})
}
func TestUpdateUserSettingsFailure(t *testing.T) {

	store := NewPostgressUserStore(testDB)     // Pass the transaction as the DB
	authStore := NewPostgressAuthStore(testDB) // Pass the transaction as the DB
	clearTables(t)
	defer clearTables(t)
	email := "kalen@laurier.ca"
	payload := &models.AuthPayload{
		Email: &email,
		ID:    "12345",
	}

	store.WithTx(context.Background(), func(tx *sql.Tx) error {

		user, err := authStore.UpsertUserWithAuth(context.Background(), tx, payload)
		if err != nil {
			t.Fatalf("Failed to insert new user: %v", err)
		}

		if payload.Email != nil && user.Email != nil && *user.Email != *payload.Email {
			t.Errorf("Expected email %s, got %s", *payload.Email, *user.Email)
		}
		settings := &models.UserSettings{
			CaloriesTarget: 3000,
			ProteinTarget:  150,
			FatTarget:      25,
			CarbsTarget:    400,
			Units:          "metric",
			Theme:          "dark",
			UpdatedAt:      time.Now().Add(time.Minute * 10),
			DeletedAt:      nil,
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
	assert.Equal(t, expected.DeletedAt != nil, actual.DeletedAt != nil, "DeletedAt 'Valid' state mismatch")

	if expected.DeletedAt != nil {
		// Only compare the time values if both are actually set (not NULL)
		assert.True(t, expected.DeletedAt.Truncate(time.Microsecond).Equal(actual.DeletedAt.Truncate(time.Microsecond)),
			"DeletedAt Time mismatch: expected %v, got %v", expected.DeletedAt, actual.DeletedAt)
	}
}
func TestUpsertUserWithAuth(t *testing.T) {
	// Start a transaction to keep the test "atomic"
	store := NewPostgressAuthStore(testDB) // Pass the transaction as the DB
	clearTables(t)
	defer clearTables(t)
	email := "kalen@laurier.ca"
	payload := &models.AuthPayload{
		Email: &email,
		ID:    "12345",
	}

	store.WithTx(context.Background(), func(tx *sql.Tx) error {

		user, err := store.UpsertUserWithAuth(context.Background(), tx, payload)
		if err != nil {
			t.Fatalf("Failed to insert new user: %v", err)
		}

		if payload.Email != nil && user.Email != nil && *user.Email != *payload.Email {
			t.Errorf("Expected email %s, got %s", *payload.Email, *user.Email)
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

func TestFirebaseIdLookup(t *testing.T) {
	// Start a transaction to keep the test "atomic"
	store := NewPostgressAuthStore(testDB) // Pass the transaction as the DB
	clearTables(t)
	defer clearTables(t)
	email := "kalen@laurier.ca"
	payload := &models.AuthPayload{
		Email: &email,
		ID:    "12345",
	}
	email2 := "hermalin@laurier.ca"

	payload2 := &models.AuthPayload{
		Email: &email2,
		ID:    "56789",
	}
	var user *models.User
	t.Run("FirebaselookupSuccess", func(t *testing.T) {
		store.WithTx(context.Background(), func(tx *sql.Tx) error {
			var err error
			user, err = store.UpsertUserWithAuth(context.Background(), tx, payload)
			if err != nil {
				t.Fatalf("Failed to insert new user: %v", err)
			}

			if payload.Email != nil && user.Email != nil && *user.Email != *payload.Email {
				t.Errorf("Expected email %s, got %s", *payload.Email, *user.Email)
			}
			_, err = store.UpsertUserWithAuth(context.Background(), tx, payload2)
			if err != nil {
				t.Fatalf("Failed to insert new user2: %v", err)
			}

			return nil

		})

		user2, err := store.LookupFirebaseIdToUserId(context.Background(), payload.ID)
		if err != nil {
			t.Fatalf("Error looking up firebase id: %v", err)
		}

		if user.ID != user2 {
			t.Error("Expected same user ID for firebase id lookup, but got a new one")
		}
	})

	t.Run("FirebaselookupFailure", func(t *testing.T) {
		store.WithTx(context.Background(), func(tx *sql.Tx) error {
			var err error
			user, err = store.UpsertUserWithAuth(context.Background(), tx, payload)
			if err != nil {
				t.Fatalf("Failed to insert new user: %v", err)
			}

			if payload.Email != nil && user.Email != nil && *user.Email != *payload.Email {
				t.Errorf("Expected email %s, got %s", *payload.Email, *user.Email)
			}
			_, err = store.UpsertUserWithAuth(context.Background(), tx, payload2)
			if err != nil {
				t.Fatalf("Failed to insert new user2: %v", err)
			}

			return nil

		})

		_, err := store.LookupFirebaseIdToUserId(context.Background(), "asds")
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				t.Logf("failed successfully, got error: %v", err)
				return
			}
			t.Errorf("Failed badly, wrong error: %v", err)
		}
		t.Errorf("Failed, no error return even when there should be")

	})

}

func TestUserIdlookup(t *testing.T) {
	// Start a transaction to keep the test "atomic"
	store := NewPostgressAuthStore(testDB) // Pass the transaction as the DB
	clearTables(t)
	defer clearTables(t)
	email := "kalen@laurier.ca"
	payload := &models.AuthPayload{
		Email: &email,
		ID:    "12345",
	}
	email2 := "hermalin@laurier.ca"

	payload2 := &models.AuthPayload{
		Email: &email2,
		ID:    "56789",
	}
	var user *models.User
	t.Run("UseridlookupSuccess", func(t *testing.T) {
		store.WithTx(context.Background(), func(tx *sql.Tx) error {
			var err error
			user, err = store.UpsertUserWithAuth(context.Background(), tx, payload)
			if err != nil {
				t.Fatalf("Failed to insert new user: %v", err)
			}

			if payload.Email != nil && user.Email != nil && *user.Email != *payload.Email {
				t.Errorf("Expected email %s, got %s", *payload.Email, *user.Email)
			}
			_, err = store.UpsertUserWithAuth(context.Background(), tx, payload2)
			if err != nil {
				t.Fatalf("Failed to insert new user2: %v", err)
			}
			return nil

		})

		user2, err := store.LookupUserIdToFirebase(context.Background(), user.ID)
		if err != nil {
			t.Fatalf("Error looking up user id: %v", err)
		}

		if payload.ID != user2 {
			t.Error("Expected same firebase ID for userid id lookup, but got a new one")
		}
	})

	t.Run("UseridlookupFailure", func(t *testing.T) {
		store.WithTx(context.Background(), func(tx *sql.Tx) error {
			var err error
			user, err = store.UpsertUserWithAuth(context.Background(), tx, payload)
			if err != nil {
				t.Fatalf("Failed to insert new user: %v", err)
			}

			if payload.Email != nil && user.Email != nil && *user.Email != *payload.Email {
				t.Errorf("Expected email %s, got %s", *payload.Email, *user.Email)
			}
			return nil

		})

		_, err := store.LookupUserIdToFirebase(context.Background(), "4be5c100-c14c-42d3-9a27-9dcff3adbaa7")
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				t.Logf("failed successfully, got error: %v", err)
				return
			}
			t.Errorf("Failed badly, wrong error: %v", err)
		}
		t.Errorf("Failed, no error return even when there should be")
	})

}

func TestUpsertUserWeightLogSuccess(t *testing.T) {
	// Start a transaction to keep the test "atomic"
	store := NewPostgressUserStore(testDB)     // Pass the transaction as the DB
	authStore := NewPostgressAuthStore(testDB) // Pass the transaction as the DB
	clearTables(t)
	defer clearTables(t)
	email := "kalen@laurier.ca"
	payload := &models.AuthPayload{
		Email: &email,
		ID:    "12345",
	}

	store.WithTx(context.Background(), func(tx *sql.Tx) error {

		user, err := authStore.UpsertUserWithAuth(context.Background(), tx, payload)
		if err != nil {
			t.Fatalf("Failed to insert new user: %v", err)
		}

		if payload.Email != nil && user.Email != nil && *user.Email != *payload.Email {
			t.Errorf("Expected email %s, got %s", *payload.Email, *user.Email)
		}
		weightLog := &models.WeightLog{
			ID:        "b38e5cee-8726-47cb-a4b6-755f8e521818",
			WeightKg:  55,
			LogDate:   "2026-11-03",
			UpdatedAt: time.Now(),
			DeletedAt: nil,
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
			LogDate:   "2026-11-03",
			UpdatedAt: time.Now().Add(time.Minute * 10),
			DeletedAt: nil,
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
	//expectedDate := expected.LogDate.Truncate(time.Microsecond).UTC()
	//actualDate := actual.LogDate.Truncate(time.Microsecond).UTC()

	assert.Equal(t, expected.LogDate, actual.LogDate, "LogDate mismatch (truncated to microsecond)")

	expectedUpdate := expected.UpdatedAt.Truncate(time.Microsecond).UTC()
	actualUpdate := actual.UpdatedAt.Truncate(time.Microsecond).UTC()

	assert.Equal(t, expectedUpdate, actualUpdate, "UpdatedAt mismatch")

	// ... (rest of the helper)
}
func TestUpsertFoodLog(t *testing.T) {
	store := NewPostgressUserStore(testDB)     // Pass the transaction as the DB
	authStore := NewPostgressAuthStore(testDB) // Pass the transaction as the DB
	clearTables(t)
	defer clearTables(t)
	email := "kalen@laurier.ca"
	payload := &models.AuthPayload{
		Email: &email,
		ID:    "12345",
	}

	store.WithTx(context.Background(), func(tx *sql.Tx) error {

		user, err := authStore.UpsertUserWithAuth(context.Background(), tx, payload)
		if err != nil {
			t.Fatalf("Failed to insert new user: %v", err)
		}

		payload := &models.FullFoodLog{
			FoodLog: &models.FoodLog{
				ID:        "0295dff6-fef4-4339-a78f-498504507e31",
				Name:      "Chicken Burger Patty",
				Time:      time.Now().UTC().Format(time.RFC3339),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				DeletedAt: nil,
			},
			FoodLogEntries: []*models.FoodLogEntry{
				{ID: "561f8c9c-50d6-40b2-bba9-f06e1294ed99",
					LogId:     "0295dff6-fef4-4339-a78f-498504507e31",
					FoodId:    "1burger2",
					FoodName:  "Chicken Patty",
					ServingId: "345",
					Unit:      "g",
					Cal:       100,
					Fat:       7,
					Carbs:     20,
					Protein:   10,
					Quantity:  2.6,
					UpdatedAt: time.Now(),
					DeletedAt: nil,
				},
			},
		}

		err = store.UpsertUserFoodLog(context.Background(), tx, user.ID, payload)
		if err != nil {
			t.Fatalf("Error with upserting food log: %v", err)
		}
		log, err := store.GetUserFoodLogById(context.Background(), tx, user.ID, payload.FoodLog.ID)
		if err != nil {
			t.Fatalf("Error retrieving the previously upserted log: %v", err)
		}
		AssertFoodLogEqual(t, payload, log)

		return nil
	})
}

func AssertFoodLogEqual(t *testing.T, expected, actual *models.FullFoodLog) {
	t.Helper()

	if expected.FoodLog.ID != actual.FoodLog.ID {
		t.Fatalf("Log ID mistmatch. Expected: %s\t Got: %s", expected.FoodLog.ID, actual.FoodLog.ID)
	}
	if expected.FoodLog.Name != actual.FoodLog.Name {
		t.Fatalf("Log name mistmatch. Expected: %s\t Got: %s", expected.FoodLog.Name, actual.FoodLog.Name)
	}
	if expected.FoodLog.Time != actual.FoodLog.Time {
		t.Fatalf("Log time mistmatch. Expected: %s\t Got: %s", expected.FoodLog.Time, actual.FoodLog.Time)
	}
	// Using assert.Equal here will force the test to print the values on failure
	expectedDate := expected.FoodLog.CreatedAt.Truncate(time.Microsecond).UTC()
	actualDate := actual.FoodLog.CreatedAt.Truncate(time.Microsecond).UTC()
	assert.Equal(t, expectedDate, actualDate, "Log created_at mismatch (truncated to microsecond)")
	expectedUpdate := expected.FoodLog.UpdatedAt.Truncate(time.Microsecond).UTC()
	actualUpdate := actual.FoodLog.UpdatedAt.Truncate(time.Microsecond).UTC()
	assert.Equal(t, expectedUpdate, actualUpdate, "Log updatedAt mismatch")

	if len(expected.FoodLogEntries) != len(actual.FoodLogEntries) {
		t.Fatalf("Log entries count mistmatch. Expected: %d\t Got: %d", len(expected.FoodLogEntries), len(actual.FoodLogEntries))
	}

	if len(expected.FoodLogEntries) == 0 {
		if actual.FoodLogEntries == nil {
			t.Fatal("Log entries returned as nil instead of []")
		}
	}
	checked := []string{}
	for _, entry := range actual.FoodLogEntries {
		for _, expEntry := range expected.FoodLogEntries {
			if expEntry.ID == entry.ID {
				if entry.LogId != expected.FoodLog.ID {
					t.Fatalf("Entry Log ID does not match Log ID. Expected: %s\t Got: %s", expected.FoodLog.ID, entry.LogId)
				}
				if expEntry.LogId != entry.LogId {
					t.Fatalf("Entry Log ID mistmatch. Expected: %s\t Got: %s", expEntry.LogId, entry.LogId)
				}

				if expEntry.FoodId != entry.FoodId {
					t.Fatalf("Entry Food ID mistmatch. Expected: %s\t Got: %s", expEntry.FoodId, entry.FoodId)
				}
				if expEntry.ServingId != entry.ServingId {
					t.Fatalf("Entry Serving ID mistmatch. Expected: %s\t Got: %s", expEntry.ServingId, entry.ServingId)
				}
				if expEntry.FoodName != entry.FoodName {
					t.Fatalf("Entry Food Name mistmatch. Expected: %s\t Got: %s", expEntry.FoodName, entry.FoodName)
				}
				if expEntry.Unit != entry.Unit {
					t.Fatalf("Entry Unit mistmatch. Expected: %s\t Got: %s", expEntry.Unit, entry.Unit)
				}
				if expEntry.Quantity != entry.Quantity {
					t.Fatalf("Entry Quanity mistmatch. Expected: %f\t Got: %f", expEntry.Quantity, entry.Quantity)
				}
				if expEntry.Cal != entry.Cal {
					t.Fatalf("Entry Calories mistmatch. Expected: %d\t Got: %d", expEntry.Cal, entry.Cal)
				}
				if expEntry.Fat != entry.Fat {
					t.Fatalf("Entry Fat mistmatch. Expected: %f\t Got: %f", expEntry.Fat, entry.Fat)
				}
				if expEntry.Protein != entry.Protein {
					t.Fatalf("Entry Protein mistmatch. Expected: %f\t Got: %f", expEntry.Protein, entry.Protein)
				}
				if expEntry.Carbs != entry.Carbs {
					t.Fatalf("Entry Carbs mistmatch. Expected: %f\t Got: %f", expEntry.Carbs, entry.Carbs)
				}
				expectedUpdate := expEntry.UpdatedAt.Truncate(time.Microsecond).UTC()
				actualUpdate := entry.UpdatedAt.Truncate(time.Microsecond).UTC()
				assert.Equal(t, expectedUpdate, actualUpdate, "Entry updated_at mismatch (truncated to microsecond)")

				assert.Equal(t, expEntry.DeletedAt != nil, entry.DeletedAt != nil, "DeletedAt 'Valid' state mismatch")

				if expEntry.DeletedAt != nil && entry.DeletedAt != nil {
					expectedDelete := expEntry.DeletedAt.Truncate(time.Microsecond).UTC()
					actualDelete := entry.DeletedAt.Truncate(time.Microsecond).UTC()
					assert.Equal(t, expectedDelete, actualDelete, "Log deleted_at mismatch")

				}
				checked = append(checked, entry.ID)
				break
			}

		}

	}

	if len(checked) != len(actual.FoodLogEntries) {
		t.Fatalf("Only matched: %d/%d logs", len(checked), len(actual.FoodLogEntries))
	}

}

func TestMultipleUpsertFoodLog(t *testing.T) {
	store := NewPostgressUserStore(testDB)     // Pass the transaction as the DB
	authStore := NewPostgressAuthStore(testDB) // Pass the transaction as the DB
	clearTables(t)
	defer clearTables(t)
	email := "kalen@laurier.ca"
	payload := &models.AuthPayload{
		Email: &email,
		ID:    "12345",
	}

	store.WithTx(context.Background(), func(tx *sql.Tx) error {

		user, err := authStore.UpsertUserWithAuth(context.Background(), tx, payload)
		if err != nil {
			t.Fatalf("Failed to insert new user: %v", err)
		}

		payload := []*models.FullFoodLog{
			{
				FoodLog: &models.FoodLog{
					ID:        "0295dff6-fef4-4339-a78f-498504507e31",
					Name:      "Chicken Burger Patty",
					Time:      time.Now().UTC().Format(time.RFC3339),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
					DeletedAt: nil,
				},
				FoodLogEntries: []*models.FoodLogEntry{
					{ID: "561f8c9c-50d6-40b2-bba9-f06e1294ed99",
						LogId:     "0295dff6-fef4-4339-a78f-498504507e31",
						FoodId:    "1burger2",
						FoodName:  "Chicken Patty",
						ServingId: "345",
						Unit:      "g",
						Cal:       100,
						Fat:       7,
						Carbs:     20,
						Protein:   10,
						Quantity:  2.6,
						UpdatedAt: time.Now(),
						DeletedAt: nil,
					},
				},
			},
			{
				FoodLog: &models.FoodLog{
					ID:        "3cdea2c2-2567-446c-a705-b6260ea47a1b",
					Name:      "Chicken Fried Rice",
					Time:      time.Now().UTC().Format(time.RFC3339),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
					DeletedAt: nil,
				},
				FoodLogEntries: []*models.FoodLogEntry{
					{
						ID:        "c0d7d94f-9b4f-46ce-800d-2668005f07e8",
						LogId:     "3cdea2c2-2567-446c-a705-b6260ea47a1b",
						FoodId:    "1friedRice2",
						FoodName:  "Fried Rice",
						ServingId: "345",
						Unit:      "g",
						Cal:       400,
						Fat:       17,
						Carbs:     42,
						Protein:   10,
						Quantity:  2.6,
						UpdatedAt: time.Now(),
						DeletedAt: nil,
					},
					{
						ID:        "abc3f919-dcbb-496e-b618-ca8766501840",
						LogId:     "3cdea2c2-2567-446c-a705-b6260ea47a1b",
						FoodId:    "1chicken2",
						FoodName:  "Grild Chicken",
						ServingId: "345",
						Unit:      "g",
						Cal:       150,
						Fat:       9,
						Carbs:     20,
						Protein:   22,
						Quantity:  8.6,
						UpdatedAt: time.Now(),
						DeletedAt: nil,
					},
				},
			},
			{
				FoodLog: &models.FoodLog{
					ID:        "b6c7e6c0-4616-4c57-86cd-35bf6b9a41c5",
					Name:      "Strawberries",
					Time:      time.Now().UTC().Format(time.RFC3339),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
					DeletedAt: nil,
				},
				FoodLogEntries: []*models.FoodLogEntry{},
			},
		}

		for _, log := range payload {
			err = store.UpsertUserFoodLog(context.Background(), tx, user.ID, log)
			if err != nil {
				t.Fatalf("Error with upserting food log: %v", err)
			}
		}
		logs, err := store.GetUserFoodLogs(context.Background(), tx, user.ID)
		if err != nil {
			t.Fatalf("Error retrieving the previously upserted log: %v", err)
		}
		if len(logs) != len(payload) {
			t.Fatalf("Not all logs came back. Expected: %d\t Got: %d", len(payload), len(logs))
		}

		for _, actual := range logs {
			for _, expected := range payload {
				if expected.FoodLog.ID == actual.FoodLog.ID {
					AssertFoodLogEqual(t, expected, actual)
				}
			}
		}
		return nil
	})
}
func TestUpsertExerciseLog(t *testing.T) {
	store := NewPostgressUserStore(testDB)     // Pass the transaction as the DB
	authStore := NewPostgressAuthStore(testDB) // Pass the transaction as the DB
	clearTables(t)
	defer clearTables(t)
	email := "kalen@laurier.ca"
	payload := &models.AuthPayload{
		Email: &email,
		ID:    "12345",
	}

	store.WithTx(context.Background(), func(tx *sql.Tx) error {

		user, err := authStore.UpsertUserWithAuth(context.Background(), tx, payload)
		if err != nil {
			t.Fatalf("Failed to insert new user: %v", err)
		}

		exerciseLog := &models.FullExerciseLog{
			ExerciseLog: &models.ExerciseLog{
				ID:         "7a1d3c52-8e4b-4f0a-9c61-2b5e8d7f3a10",
				ExerciseId: "bench_press",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
				DeletedAt:  nil,
			},
			ExerciseSets: []*models.ExerciseSet{
				{
					ID:         "c4e2a9b1-5d3f-4e8a-b7c6-1f0d9e2a8b34",
					LogId:      "7a1d3c52-8e4b-4f0a-9c61-2b5e8d7f3a10",
					Set_number: 1,
					Weight:     60,
					Reps:       10,
					UpdatedAt:  time.Now(),
					DeletedAt:  nil,
				},
			},
		}

		err = store.UpsertUserExerciseLog(context.Background(), tx, user.ID, exerciseLog)
		if err != nil {
			t.Fatalf("Error with upserting exercise log: %v", err)
		}
		log, err := store.GetUserExerciseLogById(context.Background(), tx, user.ID, exerciseLog.ExerciseLog.ID)
		if err != nil {
			t.Fatalf("Error retrieving the previously upserted log: %v", err)
		}
		AssertExerciseLogEqual(t, exerciseLog, log)

		return nil
	})
}

func AssertExerciseLogEqual(t *testing.T, expected, actual *models.FullExerciseLog) {
	t.Helper()

	if expected.ExerciseLog.ID != actual.ExerciseLog.ID {
		t.Fatalf("Log ID mistmatch. Expected: %s\t Got: %s", expected.ExerciseLog.ID, actual.ExerciseLog.ID)
	}
	if expected.ExerciseLog.ExerciseId != actual.ExerciseLog.ExerciseId {
		t.Fatalf("Log exercise ID mistmatch. Expected: %s\t Got: %s", expected.ExerciseLog.ExerciseId, actual.ExerciseLog.ExerciseId)
	}
	// Using assert.Equal here will force the test to print the values on failure
	expectedDate := expected.ExerciseLog.CreatedAt.Truncate(time.Microsecond).UTC()
	actualDate := actual.ExerciseLog.CreatedAt.Truncate(time.Microsecond).UTC()
	assert.Equal(t, expectedDate, actualDate, "Log created_at mismatch (truncated to microsecond)")
	expectedUpdate := expected.ExerciseLog.UpdatedAt.Truncate(time.Microsecond).UTC()
	actualUpdate := actual.ExerciseLog.UpdatedAt.Truncate(time.Microsecond).UTC()
	assert.Equal(t, expectedUpdate, actualUpdate, "Log updatedAt mismatch")

	assert.Equal(t, expected.ExerciseLog.DeletedAt != nil, actual.ExerciseLog.DeletedAt != nil, "Log DeletedAt 'Valid' state mismatch")
	if expected.ExerciseLog.DeletedAt != nil && actual.ExerciseLog.DeletedAt != nil {
		expectedDelete := expected.ExerciseLog.DeletedAt.Truncate(time.Microsecond).UTC()
		actualDelete := actual.ExerciseLog.DeletedAt.Truncate(time.Microsecond).UTC()
		assert.Equal(t, expectedDelete, actualDelete, "Log deleted_at mismatch")
	}

	if len(expected.ExerciseSets) != len(actual.ExerciseSets) {
		t.Fatalf("Log sets count mistmatch. Expected: %d\t Got: %d", len(expected.ExerciseSets), len(actual.ExerciseSets))
	}

	if len(expected.ExerciseSets) == 0 {
		if actual.ExerciseSets == nil {
			t.Fatal("Log sets returned as nil instead of []")
		}
	}
	checked := []string{}
	for _, set := range actual.ExerciseSets {
		for _, expSet := range expected.ExerciseSets {
			if expSet.ID == set.ID {
				if set.LogId != expected.ExerciseLog.ID {
					t.Fatalf("Set Log ID does not match Log ID. Expected: %s\t Got: %s", expected.ExerciseLog.ID, set.LogId)
				}
				if expSet.LogId != set.LogId {
					t.Fatalf("Set Log ID mistmatch. Expected: %s\t Got: %s", expSet.LogId, set.LogId)
				}
				if expSet.Set_number != set.Set_number {
					t.Fatalf("Set number mistmatch. Expected: %d\t Got: %d", expSet.Set_number, set.Set_number)
				}
				if expSet.Weight != set.Weight {
					t.Fatalf("Set weight mistmatch. Expected: %f\t Got: %f", expSet.Weight, set.Weight)
				}
				if expSet.Reps != set.Reps {
					t.Fatalf("Set reps mistmatch. Expected: %d\t Got: %d", expSet.Reps, set.Reps)
				}
				expectedUpdate := expSet.UpdatedAt.Truncate(time.Microsecond).UTC()
				actualUpdate := set.UpdatedAt.Truncate(time.Microsecond).UTC()
				assert.Equal(t, expectedUpdate, actualUpdate, "Set updated_at mismatch (truncated to microsecond)")

				assert.Equal(t, expSet.DeletedAt != nil, set.DeletedAt != nil, "Set DeletedAt 'Valid' state mismatch")
				if expSet.DeletedAt != nil && set.DeletedAt != nil {
					expectedDelete := expSet.DeletedAt.Truncate(time.Microsecond).UTC()
					actualDelete := set.DeletedAt.Truncate(time.Microsecond).UTC()
					assert.Equal(t, expectedDelete, actualDelete, "Set deleted_at mismatch")
				}
				checked = append(checked, set.ID)
				break
			}
		}
	}

	if len(checked) != len(actual.ExerciseSets) {
		t.Fatalf("Only matched: %d/%d sets", len(checked), len(actual.ExerciseSets))
	}
}

func TestMultipleUpsertExerciseLog(t *testing.T) {
	store := NewPostgressUserStore(testDB)     // Pass the transaction as the DB
	authStore := NewPostgressAuthStore(testDB) // Pass the transaction as the DB
	clearTables(t)
	defer clearTables(t)
	email := "kalen@laurier.ca"
	payload := &models.AuthPayload{
		Email: &email,
		ID:    "12345",
	}

	store.WithTx(context.Background(), func(tx *sql.Tx) error {

		user, err := authStore.UpsertUserWithAuth(context.Background(), tx, payload)
		if err != nil {
			t.Fatalf("Failed to insert new user: %v", err)
		}

		deletedAt := time.Now()
		exerciseLogs := []*models.FullExerciseLog{
			{
				ExerciseLog: &models.ExerciseLog{
					ID:         "7a1d3c52-8e4b-4f0a-9c61-2b5e8d7f3a10",
					ExerciseId: "bench_press",
					CreatedAt:  time.Now(),
					UpdatedAt:  time.Now(),
					DeletedAt:  nil,
				},
				ExerciseSets: []*models.ExerciseSet{
					{
						ID:         "c4e2a9b1-5d3f-4e8a-b7c6-1f0d9e2a8b34",
						LogId:      "7a1d3c52-8e4b-4f0a-9c61-2b5e8d7f3a10",
						Set_number: 1,
						Weight:     60,
						Reps:       10,
						UpdatedAt:  time.Now(),
						DeletedAt:  nil,
					},
				},
			},
			{
				ExerciseLog: &models.ExerciseLog{
					ID:         "e9b84f21-3c7a-4d5e-8f12-6a0b9c4d7e55",
					ExerciseId: "squat",
					CreatedAt:  time.Now(),
					UpdatedAt:  time.Now(),
					DeletedAt:  nil,
				},
				ExerciseSets: []*models.ExerciseSet{
					{
						ID:         "1b6f0d3e-9a2c-4b7e-a5d8-3c9e1f4a6b20",
						LogId:      "e9b84f21-3c7a-4d5e-8f12-6a0b9c4d7e55",
						Set_number: 1,
						Weight:     100,
						Reps:       5,
						UpdatedAt:  time.Now(),
						DeletedAt:  nil,
					},
					{
						ID:         "f3a7c2d9-6e1b-4c8f-9d40-8b2e5a7c1d63",
						LogId:      "e9b84f21-3c7a-4d5e-8f12-6a0b9c4d7e55",
						Set_number: 2,
						Weight:     102.5,
						Reps:       5,
						UpdatedAt:  time.Now(),
						DeletedAt:  nil,
					},
					{
						ID:         "5d2e8b4a-7f3c-4a9d-b1e6-0c7f2a9d4e81",
						LogId:      "e9b84f21-3c7a-4d5e-8f12-6a0b9c4d7e55",
						Set_number: 3,
						Weight:     105,
						Reps:       3,
						UpdatedAt:  time.Now(),
						DeletedAt:  &deletedAt,
					},
				},
			},
			{
				ExerciseLog: &models.ExerciseLog{
					ID:         "2c8d5a7e-4b1f-4e3a-9c6d-7f0e2b8a5c19",
					ExerciseId: "deadlift",
					CreatedAt:  time.Now(),
					UpdatedAt:  time.Now(),
					DeletedAt:  nil,
				},
				ExerciseSets: []*models.ExerciseSet{},
			},
		}

		for _, log := range exerciseLogs {
			err = store.UpsertUserExerciseLog(context.Background(), tx, user.ID, log)
			if err != nil {
				t.Fatalf("Error with upserting exercise log: %v", err)
			}
		}
		logs, err := store.GetUserExerciseLogs(context.Background(), tx, user.ID)
		if err != nil {
			t.Fatalf("Error retrieving the previously upserted logs: %v", err)
		}
		if len(logs) != len(exerciseLogs) {
			t.Fatalf("Not all logs came back. Expected: %d\t Got: %d", len(exerciseLogs), len(logs))
		}

		for _, actual := range logs {
			for _, expected := range exerciseLogs {
				if expected.ExerciseLog.ID == actual.ExerciseLog.ID {
					AssertExerciseLogEqual(t, expected, actual)
				}
			}
		}
		return nil
	})
}

func clearTables(t *testing.T) {
	// RESTART IDENTITY resets your auto-incrementing IDs back to 1
	// CASCADE handles the foreign key dependencies automatically
	query := `TRUNCATE users RESTART IDENTITY CASCADE`
	_, err := testDB.Exec(query)
	if err != nil {
		t.Fatalf("Failed to clear test database: %v", err)
	}
}
