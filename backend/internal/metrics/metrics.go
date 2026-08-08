// Package metrics — метрики Prometheus (MNT7): HTTP и бизнес-показатели
// тренажёра. Регистрируются один раз через promauto, экспортируются на
// /metrics (см. router.go). Эндпоинт стоит закрыть на уровне сети/ingress,
// а не в коде — это забота инфраструктуры, не приложения.
package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Число HTTP-запросов по маршруту, методу и коду ответа.",
	}, []string{"method", "route", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Длительность обработки HTTP-запроса.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})

	// SessionsStartedTotal и SessionsCompletedTotal считают попытки
	// независимо от типа владельца (юзер/гость): аналитика по сценариям
	// не должна занижаться из-за того, что часть игроков не авторизована.
	SessionsStartedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "training_sessions_started_total",
		Help: "Число начатых попыток прохождения по сценарию.",
	}, []string{"scenario_code"})

	SessionsCompletedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "training_sessions_completed_total",
		Help: "Число завершённых попыток по сценарию и итоговому уровню.",
	}, []string{"scenario_code", "level"})

	SessionScorePercent = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "training_session_score_percent",
		Help:    "Распределение итогового процента завершённых попыток.",
		Buckets: []float64{0, 20, 40, 50, 60, 70, 80, 90, 100},
	}, []string{"scenario_code"})
)

// ObserveHTTP пишет счётчик и гистограмму запроса. route — шаблон маршрута
// из chi (RoutePattern), а не сырой URL.Path: иначе /sessions/{uuid} даст
// неограниченную кардинальность меток.
func ObserveHTTP(method, route string, status int, started time.Time) {
	HTTPRequestsTotal.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	HTTPRequestDuration.WithLabelValues(method, route).Observe(time.Since(started).Seconds())
}
