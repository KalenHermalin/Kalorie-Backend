package main

import (
	"context"
	"database/sql"
	"log"
	"log/slog"
	"main/internal/auth/providers"
	"main/internal/handlers"
	"main/internal/llm"
	"main/internal/service"
	"main/internal/store"
	"os"
	"time"

	_ "github.com/lib/pq" // The underscore is required
	"github.com/pressly/goose/v3"
)

func main() {
	model, ok := os.LookupEnv("LLM_MODEL")
	if !ok {

		slog.Error("Error: Missing env variable", "key", "LLM_MODEL")
		os.Exit(1)
	}
	handler := slog.NewJSONHandler(os.Stdout, nil)
	logger := slog.New(handler)
	slog.SetDefault(logger)
	// Setting Up LLM Provider, Service and Handler
	apiKey, ok := os.LookupEnv("LLM_API_KEY")
	if !ok {
		slog.Error("Error: Missing env variable", "key", "LLM_API_KEY")
		os.Exit(1)

	}
	gemini, err := llm.NewGeminiProvider(context.Background(), apiKey, model)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	llmService := service.NewLLMService(gemini)
	llmHandler := handlers.NewLLMHandler(*llmService)

	// Setting up User Store, Service and Handler
	connString, ok := os.LookupEnv("DB_DNS")
	if !ok {
		slog.Error("Error: Missing env variable", "key", "DB_DNS")
		os.Exit(1)
	}
	db, err := sql.Open("postgres", connString)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	CheckDBConnection(db)
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal("Couldnt set goose dialect:", err.Error())
	}
	if err := goose.Up(db, "./migrations"); err != nil {
		slog.Error("Error: Migrations Failed", "message", err.Error())
		os.Exit(1)
	}
	userStore := store.NewPostgressUserStore(db)
	gitHubClientID, ok := os.LookupEnv("GIT_CLIENT_ID")
	if !ok {
		slog.Error("Error: Missing env variable", "key", "GIT_CLIENT_ID")
		os.Exit(1)
	}
	githubClientSecret, ok := os.LookupEnv("GIT_CLIENT_SECRET")
	if !ok {
		slog.Error("Error: Missing env variable", "key", "GIT_CLIENT_SECRET")
		os.Exit(1)
	}

	githubAuth := auth.NewGitHubProvider(gitHubClientID, githubClientSecret, "kalorie://", nil)

	googleClientIDIos, ok := os.LookupEnv("GOOGLE_CLIENT_ID_IOS")
	if !ok {
		slog.Error("Error: Missing env variable", "key", "GOOGLE_CLIENT_ID_IOS")
		os.Exit(1)
	}
	googleClientIdAndroid, ok := os.LookupEnv("GOOGLE_CLIENT_ID_ANDROID")
	if !ok {
		slog.Error("Error: Missing env variable", "key", "GOOGLE_CLIENT_ID_ANDROID")
		os.Exit(1)
	}
	googleIosAuth := auth.NewGoogleProvider(googleClientIDIos, "", "com.googleusercontent.apps.725051057596-1jsdj1vob2v5mnrhbhi1moj8bfj12cqs://", "ios", nil)
	googleAndroidAuth := auth.NewGoogleProvider(googleClientIdAndroid, "", "com.googleusercontent.apps.725051057596-gj4kl9f3c4f10cef513qsgahjppuhoqg://", "android", nil)

	jwtAccessSecret, ok := os.LookupEnv("JWT_ACCESS_SECRET")
	if !ok {

		slog.Error("Error: Missing env variable", "key", "JWT_ACCESS_SECRET")
		os.Exit(1)
	}
	jwtRefreshSecret, ok := os.LookupEnv("JWT_REFRESH_SECRET")
	if !ok {

		slog.Error("Error: Missing env variable", "key", "JWT_REFRESH_SECRET")
		os.Exit(1)
	}
	authService := service.NewAuthService(userStore, jwtAccessSecret, jwtRefreshSecret, githubAuth, googleIosAuth, googleAndroidAuth)
	authHandler := handlers.NewAuthHandler(*authService)

	// Setting up System Handler
	systemHandler := handlers.NewSystemHander()
	addr, ok := os.LookupEnv("PORT")
	if !ok {
		slog.Error("Error: Missing env variable", "key", "PORT")
		os.Exit(1)
	}
	// Initalizing Application
	app := application{
		config: config{
			addr:             "0.0.0.0:" + addr,
			jwtAccessSecret:  jwtAccessSecret,
			jwtRefreshSecret: jwtRefreshSecret,
		},
		authHandler:   *authHandler,
		llmHandler:    *llmHandler,
		systemHandler: *systemHandler,
	}
	// Running Application
	mux := app.mount()
	log.Fatal(app.run(mux))
}

func CheckDBConnection(db *sql.DB) {
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

type Nutrients struct {
	Cal     uint16 `json:"cal"`
	Carbs   uint16 `json:"carbs"`
	Fat     uint16 `json:"fat"`
	Protein uint16 `json:"protein"`
}
type Portion struct {
	Label       string `json:"label"`
	WeightGrams uint16 `json:"weight_grams"`
}
type searchResponse struct {
	Source           string    `json:"source"`
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	NutrientsPer100g Nutrients `json:"nutrients"`
	Portions         []Portion `json:"portions"`
}
