package main

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"main/internal/handlers"
	"main/internal/middlewares"
	"net/http"
	"time"
)

type application struct {
	config        config
	authHandler   handlers.AuthHandler
	llmHandler    handlers.LLMHandler
	systemHandler handlers.SystemHandler
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
	mux.Post("/auth/login", app.authHandler.HandleLoginSignup)
	mux.Post("/auth/refresh", app.authHandler.HandleRefresh)
	mux.Get("/system/health", app.systemHandler.HealthHandler)
	mux.Route("/auth", func(r chi.Router) {
		r.Use(middlewares.Auth(app.config.jwtAccessSecret))
		r.Post("/logout", app.authHandler.HandleLogOut)
	})
	mux.Route("/v1", func(r chi.Router) {
		r.Use(middlewares.Auth(app.config.jwtAccessSecret))

		r.Post("/analyze/food", app.llmHandler.AnalyzeFoodHandler)
		r.Post("/analyze/label", app.llmHandler.AnalyzeLabelHandler)
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
