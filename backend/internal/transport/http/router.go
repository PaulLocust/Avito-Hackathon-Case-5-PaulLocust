package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/config"
)

func NewRouter(handler *Handler, cfg config.Config, log *slog.Logger) http.Handler {
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(requestIDMiddleware)
	router.Use(recoverMiddleware(log))
	router.Use(loggingMiddleware(log))
	router.Use(metricsMiddleware)
	router.Use(middleware.Timeout(30 * time.Second))
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.HTTP.AllowedOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-Id"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: true, // refresh- и гостевая cookie передаются между frontend и API.
		MaxAge:           300,
	}))

	router.Get("/healthz", handler.Liveness)
	router.Get("/readyz", handler.Readiness)
	// /metrics не защищён на уровне приложения намеренно: закрывать его
	// (network policy, ingress allowlist на Prometheus scrape) — задача
	// инфраструктуры, а не бэкенда (MNT7).
	router.Handle("/metrics", promhttp.Handler())

	router.Route("/api/v1", func(api chi.Router) {
		api.Route("/auth", func(auth chi.Router) {
			auth.Post("/register", handler.Register)
			auth.Post("/login", handler.Login)
			auth.Post("/refresh", handler.RefreshToken)

			auth.Group(func(protected chi.Router) {
				protected.Use(handler.requireAuth)
				protected.Post("/logout", handler.Logout)
				protected.Get("/me", handler.CurrentUser)
			})
		})

		// Витрина, справочник — доступны и гостю, и юзеру.
		api.Group(func(public chi.Router) {
			public.Use(handler.optionalAuth)
			public.Get("/scenarios", handler.ListScenarios)
			public.Get("/scenarios/{scenarioCode}", handler.GetScenario)
			public.Get("/risk-signals", handler.ListRiskSignals)
			public.Get("/risk-signals/{signalCode}", handler.GetRiskSignal)
		})

		// Прохождение сценариев доступно и гостю, и пользователю: сессии и
		// ответы пишутся под user_id либо guest_session_id — аналитика
		// собирается независимо от регистрации.
		api.Group(func(guestOK chi.Router) {
			guestOK.Use(handler.requireOwner)
			guestOK.Post("/sessions", handler.StartSession)
			guestOK.Get("/sessions/{sessionId}", handler.GetSession)
			guestOK.Post("/sessions/{sessionId}/answers", handler.SubmitAnswer)
			guestOK.Get("/sessions/{sessionId}/result", handler.GetSessionResult)
			guestOK.Post("/sessions/{sessionId}/abandon", handler.AbandonSession)
		})

		// Аналитика — только для авторизованных: гость видит свой прогресс
		// только после регистрации/входа, когда ClaimGuest переносит его
		// сессии на аккаунт (см. handler_auth.go claimGuest). До этого
		// момента данные копятся, но наружу через этот API не отдаются.
		api.Group(func(protected chi.Router) {
			protected.Use(handler.requireAuth)
			protected.Get("/scenarios/{scenarioCode}/attempts", handler.ListAttempts)
			protected.Get("/progress", handler.GetProgress)
			protected.Get("/progress/signals", handler.GetSignalProgress)
		})
	})

	router.NotFound(handler.NotFound)
	router.MethodNotAllowed(handler.NotFound)

	return router
}
