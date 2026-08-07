package security

import (
	"net/http"
	"time"
)

const (
	RefreshCookieName = "refresh_token"

	GuestCookieName = "guest_session"
)

type CookieConfig struct {
	Domain string

	Secure bool

	SameSite http.SameSite
}

func SetRefreshCookie(
	w http.ResponseWriter,
	cfg CookieConfig,
	token string,
	exp time.Time,
) {

	http.SetCookie(w, &http.Cookie{
		Name: RefreshCookieName,

		Value: token,

		Path: "/api/v1/auth",

		Expires: exp,

		HttpOnly: true,

		Secure: cfg.Secure,

		SameSite: cfg.SameSite,

		Domain: cfg.Domain,
	})
}

func SetGuestCookie(
	w http.ResponseWriter,
	cfg CookieConfig,
	token string,
	exp time.Time,
) {

	http.SetCookie(w, &http.Cookie{
		Name: GuestCookieName,

		Value: token,

		Path: "/",

		Expires: exp,

		HttpOnly: true,

		Secure: cfg.Secure,

		SameSite: cfg.SameSite,

		Domain: cfg.Domain,
	})
}

func ClearCookie(
	w http.ResponseWriter,
	cfg CookieConfig,
	name string,
	path string,
) {

	http.SetCookie(w, &http.Cookie{
		Name: name,

		Value: "",

		Path: path,

		MaxAge: -1,

		HttpOnly: true,

		Secure: cfg.Secure,

		SameSite: cfg.SameSite,

		Domain: cfg.Domain,
	})
}

func ClearRefreshCookie(
	w http.ResponseWriter,
	cfg CookieConfig,
) {
	ClearCookie(
		w,
		cfg,
		RefreshCookieName,
		"/api/v1/auth",
	)
}

func ClearGuestCookie(
	w http.ResponseWriter,
	cfg CookieConfig,
) {
	ClearCookie(
		w,
		cfg,
		GuestCookieName,
		"/",
	)
}
