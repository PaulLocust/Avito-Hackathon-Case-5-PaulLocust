package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/logger"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/transport/http/dto"
)

// ListScenarios — GET /api/v1/scenarios.
func (h *Handler) ListScenarios(w http.ResponseWriter, r *http.Request) {
	role, err := parseRole(r.URL.Query().Get("role"))
	if err != nil {
		writeError(w, r, err)
		return
	}

	cards, err := h.services.Catalog.List(r.Context(), optionalUserID(r), role)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, logger.FromContext(r.Context(), h.log), http.StatusOK, dto.NewScenarioList(cards))
}

// GetScenario — GET /api/v1/scenarios/{scenarioCode}.
func (h *Handler) GetScenario(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "scenarioCode")

	card, err := h.services.Catalog.Get(r.Context(), optionalUserID(r), code)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, logger.FromContext(r.Context(), h.log), http.StatusOK, dto.NewScenarioDetail(card))
}

// ListAttempts — GET /api/v1/scenarios/{scenarioCode}/attempts. Только для
// авторизованных: как и /progress, доступно только после регистрации.
func (h *Handler) ListAttempts(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.ErrUnauthorized)
		return
	}

	attempts, err := h.services.Progress.Attempts(r.Context(), domain.UserOwner(user.ID), chi.URLParam(r, "scenarioCode"))
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, logger.FromContext(r.Context(), h.log), http.StatusOK, dto.NewAttemptList(attempts))
}
