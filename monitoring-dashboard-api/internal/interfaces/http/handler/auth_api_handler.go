package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dreschagin/monitoring-dashboard/internal/interfaces/http/middleware"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
)

type AuthAPIHandler struct {
	authConfig middleware.AuthConfig
	logger     *logger.Logger
}

type authLoginRequest struct {
	Token string `json:"token"`
}

func NewAuthAPIHandler(authConfig middleware.AuthConfig, log *logger.Logger) *AuthAPIHandler {
	return &AuthAPIHandler{
		authConfig: authConfig,
		logger:     log,
	}
}

func (h *AuthAPIHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.authConfig.Enabled {
		middleware.WriteJSON(w, http.StatusOK, map[string]any{
			"success":      true,
			"auth_enabled": false,
		})
		return
	}

	defer r.Body.Close()
	var req authLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	token := strings.TrimSpace(req.Token)
	if token == "" || token != h.authConfig.BearerToken {
		h.logger.Warn("Auth login failed", "remote_addr", r.RemoteAddr)
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	secureCookie := r.TLS != nil
	middleware.WriteAuthCookie(w, token, secureCookie, 12*60*60)

	middleware.WriteJSON(w, http.StatusOK, map[string]any{
		"success":      true,
		"auth_enabled": true,
	})
}

func (h *AuthAPIHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	secureCookie := r.TLS != nil
	middleware.ClearAuthCookie(w, secureCookie)
	middleware.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}

func (h *AuthAPIHandler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := middleware.ValidateRequestAuth(r, h.authConfig)
	middleware.WriteJSON(w, http.StatusOK, map[string]any{
		"auth_enabled":   h.authConfig.Enabled,
		"authenticated":  err == nil,
		"cookie_present": hasAuthCookie(r),
	})
}

func hasAuthCookie(r *http.Request) bool {
	c, err := r.Cookie(middleware.AuthCookieName)
	if err != nil {
		return false
	}
	return strings.TrimSpace(c.Value) != ""
}
