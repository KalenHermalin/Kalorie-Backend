package main

import (
	"context"
	"log"
	"log/slog"
	auth "main/internal/auth/providers"
	"main/internal/database"
	"main/internal/handlers"
	"main/internal/llm"
	"main/internal/models"
	"main/internal/service"
	"main/internal/store"
	"os"

	_ "github.com/lib/pq" // The underscore is required
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

	db := database.ConnectDatabase()
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

	// Unlike the other providers, Apple credentials are optional at boot:
	// there may not be real ones configured yet, and there's no reason a
	// missing/not-yet-configured Apple integration should keep the whole
	// app from starting. If all four are set, Apple sign-in is wired in
	// exactly like the others; otherwise "provider": "apple" on
	// /auth/login just returns ERR_INVALID_PROVIDER until they are.
	authProviders := []models.AuthProvider{githubAuth, googleIosAuth, googleAndroidAuth}
	appleClientID, appleClientIDOk := os.LookupEnv("APPLE_CLIENT_ID")
	appleTeamID, appleTeamIDOk := os.LookupEnv("APPLE_TEAM_ID")
	appleKeyID, appleKeyIDOk := os.LookupEnv("APPLE_KEY_ID")
	applePrivateKey, applePrivateKeyOk := os.LookupEnv("APPLE_PRIVATE_KEY")
	if appleClientIDOk && appleTeamIDOk && appleKeyIDOk && applePrivateKeyOk {
		// Like GitHub, this is a single provider (not split per-platform
		// like Google) - Apple issues one client id/key pair per app
		// regardless of platform.
		appleAuth, err := auth.NewAppleProvider(appleClientID, appleTeamID, appleKeyID, applePrivateKey, "kalorie://", "", nil)
		if err != nil {
			slog.Error("Error: Couldnt Initialize Apple Provider", "error", err.Error())
			os.Exit(1)
		}
		authProviders = append(authProviders, appleAuth)
	} else {
		slog.Warn("Apple auth not configured (missing one or more APPLE_* env vars) - skipping")
	}

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
	authService := service.NewAuthService(userStore, jwtAccessSecret, jwtRefreshSecret, authProviders...)
	authHandler := handlers.NewAuthHandler(*authService)

	// Setting Up User Handler
	userService := service.NewUserService(userStore)
	userHandler := handlers.NewUserHandler(*userService)
	// Setting up System Handler
	systemHandler := handlers.NewSystemHander()
	// Setting up Docs Handler (static marketing/support/privacy pages)
	docsHandler := handlers.NewDocsHandler("./docs")
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
		userHandler:   *userHandler,
		docsHandler:   *docsHandler,
	}
	// Running Application
	mux := app.mount()
	log.Fatal(app.run(mux))
}
