package main

import (
	"fmt"
	"log/slog"
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
	logger := slog.Default()
	mux.Use(middleware.RequestID)
	mux.Use(middleware.RealIP)
	mux.Use(middlewares.RequestLogger(logger))
	mux.Use(middleware.Recoverer)
	mux.Use(middleware.Timeout(time.Minute))

	// Static marketing/support/privacy pages (./docs) - public, no auth.
	mux.Get("/", app.docsHandler.IndexPage)
	mux.Get("/index.html", app.docsHandler.IndexPage)
	mux.Get("/support", app.docsHandler.SupportPage)
	mux.Get("/support.html", app.docsHandler.SupportPage)
	mux.Get("/privacy", app.docsHandler.PrivacyPage)
	mux.Get("/privacy.html", app.docsHandler.PrivacyPage)
	mux.Get("/robots.txt", app.docsHandler.RobotsTxt)
	mux.Get("/sitemap.xml", app.docsHandler.Sitemap)
	mux.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir("./docs/assets"))))

	mux.Route("/api", func(r chi.Router) {
		r.Get("/system/health", app.systemHandler.HealthHandler)
		r.Post("/waitlist", handlers.Handle(app.docsHandler.RemindMe))
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", handlers.Handle(app.authHandler.HandleLoginSignup))
			r.Post("/refresh", handlers.Handle(app.authHandler.HandleRefresh))
			r.Group(func(r chi.Router) {
				r.Use(middlewares.Auth(app.config.jwtAccessSecret))
				r.Post("/logout", handlers.Handle(app.authHandler.HandleLogOut))
				r.Delete("/account", handlers.Handle(app.authHandler.HandleDeleteUser))

			})
		})
		r.Route("/v1", func(r chi.Router) {
			r.Use(middlewares.Auth(app.config.jwtAccessSecret))

			r.Put("/settings/sync", handlers.Handle(app.userHandler.EgressSyncSettings))
			r.Get("/settings/sync", handlers.Handle(app.userHandler.IngressSyncSettings))

			r.Put("/weight-logs/sync", handlers.Handle(app.userHandler.EgressSyncWeightLogs))
			r.Get("/weight-logs/sync", handlers.Handle(app.userHandler.IngressSyncWeightLogs))

			r.Put("/food-logs/sync", handlers.Handle(app.userHandler.EgressSyncFoodLogs))
			r.Get("/food-logs/sync", handlers.Handle(app.userHandler.IngressSyncFoodLogs))

			r.Put("/exercise-logs/sync", handlers.Handle(app.userHandler.EgressSyncExerciseLogs))
			r.Get("/exercise-logs/sync", handlers.Handle(app.userHandler.IngressSyncExerciseLogs))

			r.Post("/analyze/food", handlers.Handle(app.llmHandler.AnalyzeFoodHandler))
			r.Post("/analyze/label", handlers.Handle(app.llmHandler.AnalyzeLabelHandler))
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
