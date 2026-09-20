package database

import (
	"database/sql"
	"log/slog"
	"os"
	"time"

	_ "github.com/lib/pq" // The underscore is required
)

func ConnectDatabase() *sql.DB {
	connString, ok := os.LookupEnv("DATABASE_URL")
	if !ok {
		slog.Error("Error: Missing env variable", "key", "DATABASE_URL")
		os.Exit(1)
	}
	db, err := sql.Open("postgres", connString)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	checkDBConnection(db)
	return db
}

func checkDBConnection(db *sql.DB) {
	connected := false
	for range 10 {
		err := db.Ping()
		if err == nil {
			connected = true
			break
		}
		slog.Info("Waiting for database connection...")
		time.Sleep(3 * time.Second)
	}
	if !connected {
		slog.Error("Error: Could not connect to database after 30sec")
		os.Exit(1)
	}
}
