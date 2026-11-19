package main

import (
	"context"
	"log"
	"log/slog"
	"main/internal/database"
	"main/internal/handlers"
	"main/internal/llm"
	"main/internal/service"
	"main/internal/store"
	"os"

	firebase "firebase.google.com/go/v4"
	_ "github.com/lib/pq" // The underscore is required
	"google.golang.org/api/option"
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
	geminiApiKey, ok := os.LookupEnv("GEMINI_API_KEY")
	if !ok {
		slog.Error("Error: Missing env variable", "key", "GEMINI_API_KEY")
		os.Exit(1)

	}

	gemini, err := llm.NewGeminiProvider(context.Background(), geminiApiKey, model)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	llmService := service.NewLLMService(gemini)
	llmHandler := handlers.NewLLMHandler(*llmService)
	// Setting up User Store, Service and Handler

	db := database.ConnectDatabase()
	userStore := store.NewPostgressUserStore(db)
	remindStore := store.NewRemindMeStore(db)

	firebaseServiceAccount, ok := os.LookupEnv("FIREBASE_SERVICE_ACCOUNT_JSON")
	if !ok {

		slog.Error("Error: Missing env variable", "key", "FIREBASE_SERVICE_ACCOUNT_JSON")
		os.Exit(1)
	}
	opt := option.WithAuthCredentialsJSON(option.ServiceAccount, []byte(firebaseServiceAccount))

	firebaseClient, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		slog.Error("Error Initializng the firebase client", "error", err)
		os.Exit(1)
	}
	authClient, err := firebaseClient.Auth(context.Background())
	if err != nil {
		slog.Error("Error: creating auth client", "error", err)
		os.Exit(1)
	}
	authStore := store.NewPostgressAuthStore(db)
	authService := service.NewAuthService(authStore, authClient)
	authHandler := handlers.NewAuthHandler(*authService)
	// Setting Up User Handler
	userService := service.NewUserService(userStore)
	userHandler := handlers.NewUserHandler(*userService)
	// Setting up System Handler
	systemHandler := handlers.NewSystemHander()
	// Setting up Docs Handler (static marketing/support/privacy pages)
	docsHandler := handlers.NewDocsHandler("./docs", remindStore)
	addr, ok := os.LookupEnv("PORT")
	if !ok {
		slog.Error("Error: Missing env variable", "key", "PORT")
		os.Exit(1)
	}
	// Initalizing Application
	app := application{
		config: config{
			addr: "0.0.0.0:" + addr,
		},
		authHandler:        *authHandler,
		llmHandler:         *llmHandler,
		systemHandler:      *systemHandler,
		userHandler:        *userHandler,
		docsHandler:        *docsHandler,
		firebaseAuthClient: authClient,
		authLookup:         authService,
	}
	// Running Application
	mux := app.mount()
	log.Fatal(app.run(mux))
}
