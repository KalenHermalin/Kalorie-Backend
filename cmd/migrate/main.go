package main

import (
	"log"
	"log/slog"
	"main/internal/database"
	"os"

	"github.com/pressly/goose/v3"
)

func main() {

	db := database.ConnectDatabase()
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal("Couldnt set goose dialect:", err.Error())
	}
	if err := goose.Up(db, "./migrations"); err != nil {
		slog.Error("Error: Migrations Failed", "message", err.Error())
		os.Exit(1)
	}

}
