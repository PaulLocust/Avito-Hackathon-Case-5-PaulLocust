package http

import (
	"net/http"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/logger"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/transport/http/dto"
)

// Liveness — GET /healthz.
func (h *Handler) Liveness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, logger.FromContext(r.Context(), h.log), http.StatusOK, dto.HealthResponse{
		Status:  "ok",
		Version: h.version,
	})
}

// Readiness — GET /readyz. Пока БД недоступна, отвечает 503.
func (h *Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context(), h.log)

	response := dto.HealthResponse{
		Status:  "ok",
		Version: h.version,
		Checks:  map[string]string{"database": "ok"},
	}

	if err := h.db.Ping(r.Context()); err != nil {
		response.Status = "degraded"
		response.Checks["database"] = "unavailable"

		writeJSON(w, log, http.StatusServiceUnavailable, response)

		return
	}

	writeJSON(w, log, http.StatusOK, response)
}

// NotFound — единый конверт ошибки на неизвестный маршрут и метод.
func (h *Handler) NotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, domain.ErrNotFound)
}
