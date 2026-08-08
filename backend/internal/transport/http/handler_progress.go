package http

import (
	"net/http"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/logger"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/transport/http/dto"
)

// GetProgress — GET /api/v1/progress. Только для авторизованных (requireAuth):
// гость видит прогресс лишь после регистрации, когда его сессии перенесены
// с guest_session_id на user_id.
func (h *Handler) GetProgress(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.ErrUnauthorized)
		return
	}

	progress, err := h.services.Progress.Overview(r.Context(), domain.UserOwner(user.ID))
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, logger.FromContext(r.Context(), h.log), http.StatusOK, dto.NewProgress(progress))
}

// GetSignalProgress — GET /api/v1/progress/signals.
func (h *Handler) GetSignalProgress(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.ErrUnauthorized)
		return
	}

	stats, err := h.services.Progress.Signals(r.Context(), domain.UserOwner(user.ID))
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, logger.FromContext(r.Context(), h.log), http.StatusOK, dto.NewSignalProgress(stats))
}
