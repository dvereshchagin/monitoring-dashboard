package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
)

var ErrUnauthorized = errors.New("unauthorized")

type AuthConfig struct {
	Enabled     bool
	BearerToken string
}

const AuthCookieName = "monitoring_auth_token"

// Auth защищает endpoint простым Bearer token механизмом.
// Это baseline-реализация для P0, которую позже можно заменить на JWT/JWKS.
func Auth(cfg AuthConfig, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := ValidateRequestAuth(r, cfg); err != nil {
				log.Warn("Unauthorized request",
					"path", r.URL.Path,
					"method", r.Method,
					"remote_addr", r.RemoteAddr,
				)
				w.Header().Set("WWW-Authenticate", `Bearer realm="monitoring-dashboard"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func ValidateRequestAuth(r *http.Request, cfg AuthConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.BearerToken) == "" {
		return ErrUnauthorized
	}

	token := ExtractToken(r)
	if token == "" || token != cfg.BearerToken {
		return ErrUnauthorized
	}

	return nil
}

func ExtractToken(r *http.Request) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}

	if c, err := r.Cookie(AuthCookieName); err == nil {
		if value := strings.TrimSpace(c.Value); value != "" {
			return value
		}
	}

	// Для WebSocket браузер не может отправить кастомный Authorization header через new WebSocket().
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

func WriteAuthCookie(w http.ResponseWriter, token string, secure bool, maxAgeSeconds int) {
	http.SetCookie(w, &http.Cookie{
		Name:     AuthCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAgeSeconds,
	})
}

func ClearAuthCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     AuthCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
