package store

import (
	"context"
	"database/sql"
	"log"
	"main/internal/auth"
	"main/internal/models"
	"testing"
	"time"

	_ "github.com/lib/pq" // Your Postgres driver
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

func clearTables(t *testing.T) {
	// RESTART IDENTITY resets your auto-incrementing IDs back to 1
	// CASCADE handles the foreign key dependencies automatically
	query := `TRUNCATE users, provider_identities, refresh_tokens RESTART IDENTITY CASCADE`
	_, err := testDB.Exec(query)
	if err != nil {
		t.Fatalf("Failed to clear test database: %v", err)
	}
}
