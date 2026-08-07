package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/logger"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/transport/http/dto"
)

// StartSession — POST /api/v1/sessions.
func (h *Handler) StartSession(w http.ResponseWriter, r *http.Request) {
	owner, ok := currentOwner(r)
	if !ok {
		writeError(w, r, domain.ErrUnauthorized)
		return
	}

	request, ok := decodeJSON[dto.StartSessionRequest](w, r)
	if !ok {
		return
	}

	snapshot, err := h.services.Training.Start(r.Context(), owner, request.ScenarioCode, request.Restart)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, logger.FromContext(r.Context(), h.log), http.StatusCreated, dto.NewSessionState(snapshot))
}

// GetSession — GET /api/v1/sessions/{sessionId}.
func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) {
	owner, ok := currentOwner(r)
	if !ok {
		writeError(w, r, domain.ErrUnauthorized)
		return
	}

	sessionID, err := parseUUID(chi.URLParam(r, "sessionId"))
	if err != nil {
		writeError(w, r, err)
		return
	}

	snapshot, err := h.services.Training.Get(r.Context(), owner, sessionID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, logger.FromContext(r.Context(), h.log), http.StatusOK, dto.NewSessionState(snapshot))
}

// SubmitAnswer — POST /api/v1/sessions/{sessionId}/answers.
func (h *Handler) SubmitAnswer(w http.ResponseWriter, r *http.Request) {
	owner, ok := currentOwner(r)
	if !ok {
		writeError(w, r, domain.ErrUnauthorized)
		return
	}

	sessionID, err := parseUUID(chi.URLParam(r, "sessionId"))
	if err != nil {
		writeError(w, r, err)
		return
	}

	request, ok := decodeJSON[dto.SubmitAnswerRequest](w, r)
	if !ok {
		return
	}

	outcome, err := h.services.Training.SubmitAnswer(
		r.Context(), owner, sessionID, request.StepCode, request.OptionCode,
	)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, logger.FromContext(r.Context(), h.log), http.StatusOK, dto.NewAnswerResult(outcome))
}

// GetSessionResult — GET /api/v1/sessions/{sessionId}/result.
func (h *Handler) GetSessionResult(w http.ResponseWriter, r *http.Request) {
	owner, ok := currentOwner(r)
	if !ok {
		writeError(w, r, domain.ErrUnauthorized)
		return
	}

	sessionID, err := parseUUID(chi.URLParam(r, "sessionId"))
	if err != nil {
		writeError(w, r, err)
		return
	}

	debrief, err := h.services.Training.Result(r.Context(), owner, sessionID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, logger.FromContext(r.Context(), h.log), http.StatusOK, dto.NewSessionResult(debrief))
}

// AbandonSession — POST /api/v1/sessions/{sessionId}/abandon.
func (h *Handler) AbandonSession(w http.ResponseWriter, r *http.Request) {
	owner, ok := currentOwner(r)
	if !ok {
		writeError(w, r, domain.ErrUnauthorized)
		return
	}

	sessionID, err := parseUUID(chi.URLParam(r, "sessionId"))
	if err != nil {
		writeError(w, r, err)
		return
	}

	if err := h.services.Training.Abandon(r.Context(), owner, sessionID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
