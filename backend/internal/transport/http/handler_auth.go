package http

import (
	"net/http"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/logger"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/transport/http/dto"
)

// Register — POST /api/v1/auth/register.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeJSON[dto.RegisterRequest](w, r)
	if !ok {
		return
	}

	user, token, err := h.services.Auth.Register(r.Context(), request.Nickname, request.Password)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, logger.FromContext(r.Context(), h.log), http.StatusCreated, dto.NewAuthResponse(user, token))
}

// Login — POST /api/v1/auth/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeJSON[dto.LoginRequest](w, r)
	if !ok {
		return
	}

	user, token, err := h.services.Auth.Login(r.Context(), request.Nickname, request.Password)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, logger.FromContext(r.Context(), h.log), http.StatusOK, dto.NewAuthResponse(user, token))
}

// Logout — POST /api/v1/auth/logout.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r)
	if !ok {
		writeError(w, r, domain.ErrUnauthorized)
		return
	}

	if err := h.services.Auth.Logout(r.Context(), token); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CurrentUser — GET /api/v1/auth/me.
func (h *Handler) CurrentUser(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUser(r)
	if !ok {
		writeError(w, r, domain.ErrUnauthorized)
		return
	}

	writeJSON(w, logger.FromContext(r.Context(), h.log), http.StatusOK, dto.NewUser(user))
}
