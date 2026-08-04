package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/logger"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/transport/http/dto"
)

// ListRiskSignals — GET /api/v1/risk-signals.
func (h *Handler) ListRiskSignals(w http.ResponseWriter, r *http.Request) {
	side, err := parseSide(r.URL.Query().Get("side"))
	if err != nil {
		writeError(w, r, err)
		return
	}

	signals, err := h.services.Reference.ListSignals(r.Context(), side)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, logger.FromContext(r.Context(), h.log), http.StatusOK, dto.NewRiskSignalList(signals))
}

// GetRiskSignal — GET /api/v1/risk-signals/{signalCode}.
func (h *Handler) GetRiskSignal(w http.ResponseWriter, r *http.Request) {
	detail, err := h.services.Reference.GetSignal(r.Context(), chi.URLParam(r, "signalCode"))
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, logger.FromContext(r.Context(), h.log), http.StatusOK, dto.NewRiskSignalDetail(detail))
}
