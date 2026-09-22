package main

import (
	"fmt"
	"main/internal/handlers"
	"main/internal/middlewares"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type application struct {
	config        config
	authHandler   handlers.AuthHandler
	llmHandler    handlers.LLMHandler
	systemHandler handlers.SystemHandler
	userHandler   handlers.UserHandler
	docsHandler   handlers.DocsHandler
}

type config struct {
	addr             string
	jwtAccessSecret  string
	jwtRefreshSecret string
}

func (app *application) mount() http.Handler {
	mux := chi.NewRouter()
	mux.Use(middleware.RequestID)
	mux.Use(middleware.RealIP)
	mux.Use(middleware.Logger)
	mux.Use(middleware.Recoverer)
	mux.Use(middleware.Timeout(time.Minute))

	// Static marketing/support/privacy pages (./docs) - public, no auth.
	mux.Get("/", app.docsHandler.IndexPage)
	mux.Get("/index.html", app.docsHandler.IndexPage)
	mux.Get("/support", app.docsHandler.SupportPage)
	mux.Get("/support.html", app.docsHandler.SupportPage)
	mux.Get("/privacy", app.docsHandler.PrivacyPage)
	mux.Get("/privacy.html", app.docsHandler.PrivacyPage)
	mux.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir("./docs/assets"))))

	mux.Route("/api", func(r chi.Router) {
		r.Get("/system/health", app.systemHandler.HealthHandler)
		r.Post("/waitlist", app.docsHandler.RemindMe)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", app.authHandler.HandleLoginSignup)
			r.Post("/refresh", app.authHandler.HandleRefresh)
			r.Group(func(r chi.Router) {
				r.Use(middlewares.Auth(app.config.jwtAccessSecret))
				r.Post("/logout", app.authHandler.HandleLogOut)

			})
		})
		r.Route("/v1", func(r chi.Router) {
			r.Use(middlewares.Auth(app.config.jwtAccessSecret))

			r.Put("/settings/sync", app.userHandler.EgressSyncSettings)
			r.Get("/settings/sync", app.userHandler.IngressSyncSettings)

			r.Put("/weight-logs/sync", app.userHandler.EgressSyncWeightLogs)
			r.Get("/weight-logs/sync", app.userHandler.IngressSyncWeightLogs)

			r.Put("/food-logs/sync", app.userHandler.EgressSyncFoodLogs)
			r.Get("/food-logs/sync", app.userHandler.IngressSyncFoodLogs)

			r.Put("/exercise-logs/sync", app.userHandler.EgressSyncExerciseLogs)
			r.Get("/exercise-logs/sync", app.userHandler.IngressSyncExerciseLogs)

			r.Post("/analyze/food", app.llmHandler.AnalyzeFoodHandler)
			r.Post("/analyze/label", app.llmHandler.AnalyzeLabelHandler)
		})
	})
	return mux
}

func (app *application) run(mux http.Handler) error {
	serv := &http.Server{
		Addr:         app.config.addr,
		Handler:      mux,
		WriteTimeout: time.Second * 30,
		ReadTimeout:  time.Second * 10,
		IdleTimeout:  time.Minute,
	}
	fmt.Printf("Server has started and is listening at %s\n", app.config.addr)
	return serv.ListenAndServe()
}
