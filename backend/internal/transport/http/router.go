package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/config"
)

// NewRouter собирает маршруты. Таблица должна совпадать с api/openapi.yaml.
func NewRouter(handler *Handler, cfg config.Config, log *slog.Logger) http.Handler {
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(requestIDMiddleware)
	router.Use(recoverMiddleware(log))
	router.Use(loggingMiddleware(log))
	router.Use(middleware.Timeout(30 * time.Second))
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.HTTP.AllowedOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-Id"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	router.Get("/healthz", handler.Liveness)
	router.Get("/readyz", handler.Readiness)

	router.Route("/api/v1", func(api chi.Router) {
		api.Route("/auth", func(auth chi.Router) {
			auth.Post("/register", handler.Register)
			auth.Post("/login", handler.Login)

			auth.Group(func(protected chi.Router) {
				protected.Use(handler.requireAuth)
				protected.Post("/logout", handler.Logout)
				protected.Get("/me", handler.CurrentUser)
			})
		})

		// Витрина и справочник доступны гостю (FR4).
		api.Group(func(public chi.Router) {
			public.Use(handler.optionalAuth)
			public.Get("/scenarios", handler.ListScenarios)
			public.Get("/scenarios/{scenarioCode}", handler.GetScenario)
			public.Get("/risk-signals", handler.ListRiskSignals)
			public.Get("/risk-signals/{signalCode}", handler.GetRiskSignal)
		})

		api.Group(func(protected chi.Router) {
			protected.Use(handler.requireAuth)

			protected.Get("/scenarios/{scenarioCode}/attempts", handler.ListAttempts)

			protected.Post("/sessions", handler.StartSession)
			protected.Get("/sessions/{sessionId}", handler.GetSession)
			protected.Post("/sessions/{sessionId}/answers", handler.SubmitAnswer)
			protected.Get("/sessions/{sessionId}/result", handler.GetSessionResult)
			protected.Post("/sessions/{sessionId}/abandon", handler.AbandonSession)

			protected.Get("/progress", handler.GetProgress)
			protected.Get("/progress/signals", handler.GetSignalProgress)
		})
	})

	router.NotFound(handler.NotFound)
	router.MethodNotAllowed(handler.NotFound)

	return router
}
