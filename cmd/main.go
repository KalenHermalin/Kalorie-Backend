package main

import (
	"context"
	"database/sql"
	"log"
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

const MODEL string = "gemini-2.5-flash-lite"

func main() {

	// Setting Up LLM Provider, Service and Handler
	apiKey, ok := os.LookupEnv("GEMINI_API_KEY")
	if !ok {
		log.Fatalln("Error getting LLM API Key from ENV")
	}
	gemini, err := llm.NewGeminiProvider(context.Background(), &apiKey, "gemini-flash-2.5")
	if err != nil {
		log.Panic(err.Error())
	}
	llmService := service.NewLLMService(gemini)
	llmHandler := handlers.NewLLMHandler(*llmService)

	// Setting up User Store, Service and Handler
	connString, ok := os.LookupEnv("DB_DNS")
	if !ok {
		log.Fatalln("Error getting DB Connection String from ENV")
	}
	db, err := sql.Open("postgres", connString)
	if err != nil {
		log.Fatal(err)
	}
	CheckDBConnection(db)
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal("Couldnt set goose dialect:", err.Error())
	}
	if err := goose.Up(db, "migrations"); err != nil {
		log.Fatal("Migration failed:", err.Error())
	}
	userStore := store.NewPostgressUserStore(db)
	gitHubClientID, ok := os.LookupEnv("GIT_CLIENT_ID")
	if !ok {
		log.Panicln("Failed to get GIT Client ID from ENV")
	}
	githubClientSecret, ok := os.LookupEnv("GIT_CLIENT_SECRET")
	if !ok {
		log.Panicln("Failed to get GIT Secert from ENV")
	}

	githubAuth := auth.NewGitHubProvider(gitHubClientID, githubClientSecret, "nutrikal://", nil)

	googleClientIDIos, ok := os.LookupEnv("GOOGLE_CLIENT_ID_IOS")
	if !ok {
		log.Panicln("Failed to get Google Client ID from ENV")
	}
	googleClientIdAndroid, ok := os.LookupEnv("GOOGLE_CLIENT_ID_ANDROID")
	if !ok {
		log.Panicln("Failed to get Google Client ID from ENV")
	}
	googleIosAuth := auth.NewGoogleProvider(googleClientIDIos, "", "com.googleusercontent.apps.725051057596-1jsdj1vob2v5mnrhbhi1moj8bfj12cqs://", "ios", nil)
	googleAndroidAuth := auth.NewGoogleProvider(googleClientIdAndroid, "", "com.googleusercontent.apps.725051057596-gj4kl9f3c4f10cef513qsgahjppuhoqg://", "android", nil)

	jwtAccessSecret, ok := os.LookupEnv("JWT_ACCESS_SECRET")
	if !ok {

		log.Panicln("Failed to get JWT_SECERT from ENV")
	}
	jwtRefreshSecret, ok := os.LookupEnv("JWT_REFRESH_SECRET")
	if !ok {

		log.Panicln("Failed to get JWT_SECERT from ENV")
	}
	authService := service.NewAuthService(*userStore, jwtAccessSecret, jwtRefreshSecret, githubAuth, googleIosAuth, googleAndroidAuth)
	authHandler := handlers.NewAuthHandler(*authService)

	// Setting up System Handler
	systemHandler := handlers.NewSystemHander()
	addr, ok := os.LookupEnv("PORT")
	if !ok {
		log.Panicln("Failed to get PORT from ENV")
	}
	// Initalizing Application
	app := application{
		config: config{
			addr:             ":" + addr,
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
		log.Println("Waiting For Database Conneciton...")
		time.Sleep(3 * time.Second)
	}
	if !connected {
		log.Fatalln("Could not connect to database after 30 seconds. Exiting.")
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
