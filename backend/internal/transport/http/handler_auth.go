// internal/transport/http/handler_auth.go
package http

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/logger"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/security"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/service"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/transport/http/dto"
)

func (h *Handler) setAuthCookies(w http.ResponseWriter, pair service.TokenPair) {
	security.SetRefreshCookie(w, h.cookieCfg, pair.Refresh.Value, pair.Refresh.ExpiresAt)
}

// claimGuest — после успешного Register/Login переносит прогресс гостя
// (по куке guest_session) на аккаунт и чистит гостевую куку. Именно это
// даёт "гость зарегался — у него появилась аналитика по своему прогрессу".
func (h *Handler) claimGuest(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	c, err := r.Cookie(security.GuestCookieName)
	if err != nil || c.Value == "" {
		return
	}
	if err := h.services.Auth.ClaimGuest(r.Context(), userID, c.Value); err != nil {
		logger.FromContext(r.Context(), h.log).Error("не удалось перенести гостевые результаты", slog.String("error", err.Error()))
		return
	}
	security.ClearGuestCookie(w, h.cookieCfg)
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context(), slog.Default())

	req, ok := decodeJSON[dto.RegisterRequest](w, r)
	if !ok {
		return
	}

	user, pair, err := h.services.Auth.Register(r.Context(), req.Nickname, req.Password)
	if err != nil {
		writeError(w, r, err)
		return
	}

	h.claimGuest(w, r, user.ID)
	h.setAuthCookies(w, pair)

	writeJSON(w, log, http.StatusCreated, dto.NewAuthResponse(user, pair.Access))
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context(), slog.Default())

	req, ok := decodeJSON[dto.LoginRequest](w, r)
	if !ok {
		return
	}

	user, pair, err := h.services.Auth.Login(r.Context(), req.Nickname, req.Password)
	if err != nil {
		writeError(w, r, err)
		return
	}

	h.claimGuest(w, r, user.ID)
	h.setAuthCookies(w, pair)

	writeJSON(w, log, http.StatusOK, dto.NewAuthResponse(user, pair.Access))
}

func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context(), slog.Default())

	c, err := r.Cookie(security.RefreshCookieName)
	if err != nil || c.Value == "" {
		writeError(w, r, domain.ErrRefreshTokenMissing)
		return
	}

	user, pair, err := h.services.Auth.Refresh(r.Context(), c.Value)
	if err != nil {
		security.ClearRefreshCookie(w, h.cookieCfg)
		writeError(w, r, err)
		return
	}

	h.setAuthCookies(w, pair)
	writeJSON(w, log, http.StatusOK, dto.NewAuthResponse(user, pair.Access))
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	access, ok := accessToken(r)
	if !ok {
		writeError(w, r, domain.ErrUnauthorized)
		return
	}

	if err := h.services.Auth.Logout(r.Context(), access); err != nil {
		writeError(w, r, err)
		return
	}
	security.ClearRefreshCookie(w, h.cookieCfg)

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CurrentUser(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context(), slog.Default())

	user, ok := currentUser(r) // FIXED: было UserFromContext (не существовало)
	if !ok {
		writeError(w, r, domain.ErrUnauthorized)
		return
	}

	writeJSON(w, log, http.StatusOK, dto.NewUser(user))
}
