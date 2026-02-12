package store

import (
	"database/sql"
	"log"
	"os"
	"testing"

	_ "github.com/lib/pq" // The underscore is required
	"github.com/pressly/goose/v3"
)

func TestMain(m *testing.M) {
	// Setup conneciton to test DB
	connString, ok := os.LookupEnv("TEST_DB_URL")
	if !ok {
		log.Fatal("Could not find TEST_DB_URL key in environment")
	}
	var err error
	testDB, err = sql.Open("postgres", connString)
	if err != nil {
		log.Fatalf("Could not connect to test databse: %v", err)
	}
	CheckDBConnection(testDB)
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal("Couldnt set goose dialect:", err.Error())
	}
	if err := goose.Up(testDB, "../../migrations"); err != nil {
		log.Fatal("Migration failed:", err.Error())
	}
	code := m.Run()
	testDB.Close()
	os.Exit(code)

}
