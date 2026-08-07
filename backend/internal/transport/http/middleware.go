package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/domain"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/logger"
	"github.com/PaulLocust/Avito-Hackathon-Case-5/backend/internal/security"
)

type userContextKey struct{}
type ownerContextKey struct{}

// withUser кладёт в контекст и юзера, и его Owner-представление — так
// requireAuth-роуты (аналитика) могут доставать domain.User, а общие
// хелперы (currentOwner) — единообразный domain.Owner.
func withUser(ctx context.Context, user domain.User) context.Context {
	ctx = context.WithValue(ctx, userContextKey{}, user)
	return context.WithValue(ctx, ownerContextKey{}, domain.UserOwner(user.ID))
}

func withOwner(ctx context.Context, owner domain.Owner) context.Context {
	return context.WithValue(ctx, ownerContextKey{}, owner)
}

func userFromContext(ctx context.Context) (domain.User, bool) {
	user, ok := ctx.Value(userContextKey{}).(domain.User)
	return user, ok
}

// ownerFromContext — владелец сессии прохождения: юзер или гость.
// Кладётся requireAuth/optionalAuth.
func ownerFromContext(ctx context.Context) (domain.Owner, bool) {
	owner, ok := ctx.Value(ownerContextKey{}).(domain.Owner)
	return owner, ok
}

// requestIDMiddleware кладёт идентификатор запроса в контекст, чтобы строки
// лога и поле request_id в ответе совпадали.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.GetReqID(r.Context())
		ctx := logger.WithRequestID(r.Context(), requestID)

		w.Header().Set("X-Request-Id", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// loggingMiddleware пишет строку на каждый запрос. Тела не логируются: в них
// приходят пароли (SEC7).
func loggingMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(wrapped, r)

			logger.FromContext(r.Context(), log).Info("запрос обработан",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", wrapped.Status()),
				slog.Int("bytes", wrapped.BytesWritten()),
				slog.Duration("duration", time.Since(started)),
			)
		})
	}
}

// recoverMiddleware не даёт панике остановить сервис (REL3).
func recoverMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}

				if err, ok := recovered.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(recovered)
				}

				logger.FromContext(r.Context(), log).Error("паника в обработчике",
					slog.Any("panic", recovered),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
				)

				writeError(w, r, errInternal)
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// accessToken достаёт JWT из заголовка Authorization. Refresh- и гостевой
// токены никогда не подходят для аутентификации запросов.
func accessToken(r *http.Request) (string, bool) {
	if header := r.Header.Get("Authorization"); header != "" {
		if token, err := security.ParseBearer(header); err == nil {
			return string(token), true
		}
	}
	return "", false
}

// requireAuth — доступ только авторизованным. Вешать на аналитику
// (/progress, /auth/me и т.п.) — гость сюда не пройдёт.
func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := accessToken(r)
		if !ok {
			writeError(w, r, domain.ErrUnauthorized)
			return
		}

		user, err := h.services.Auth.Authenticate(r.Context(), token)
		if err != nil {
			writeError(w, r, domain.ErrUnauthorized)
			return
		}

		next.ServeHTTP(w, r.WithContext(withUser(r.Context(), user)))
	})
}

// optionalAuth добавляет пользователя к публичному запросу, если JWT валиден.
// Гостевая cookie здесь не нужна: витрина не сохраняет результаты.
func (h *Handler) optionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token, ok := accessToken(r); ok {
			if user, err := h.services.Auth.Authenticate(r.Context(), token); err == nil {
				next.ServeHTTP(w, r.WithContext(withUser(r.Context(), user)))
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// requireOwner разрешает прохождение и пользователю, и гостю. Наличие
// невалидного Authorization не превращает пользователя молча в гостя: клиент
// должен сначала обновить access JWT через refresh endpoint.
func (h *Handler) requireOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			token, ok := accessToken(r)
			if !ok {
				writeError(w, r, domain.ErrUnauthorized)
				return
			}
			user, err := h.services.Auth.Authenticate(r.Context(), token)
			if err != nil {
				writeError(w, r, domain.ErrUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(withUser(r.Context(), user)))
			return
		}

		if cookie, err := r.Cookie(security.GuestCookieName); err == nil && cookie.Value != "" {
			if guestID, err := h.services.Guest.Validate(r.Context(), cookie.Value); err == nil {
				next.ServeHTTP(w, r.WithContext(withOwner(r.Context(), domain.GuestOwner(guestID))))
				return
			}
		}

		token, err := h.services.Guest.Start(r.Context())
		if err != nil {
			writeError(w, r, err)
			return
		}
		security.SetGuestCookie(w, h.cookieCfg, token.Value, token.ExpiresAt)

		next.ServeHTTP(w, r.WithContext(withOwner(r.Context(), domain.GuestOwner(token.OwnerID))))
	})
}
