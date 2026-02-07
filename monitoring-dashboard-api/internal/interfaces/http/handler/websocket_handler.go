package handler

import (
	"net/http"
	"net/url"
	"strings"

	wsInfra "github.com/dreschagin/monitoring-dashboard/internal/infrastructure/notification/websocket"
	"github.com/dreschagin/monitoring-dashboard/internal/interfaces/http/middleware"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
	"github.com/gorilla/websocket"
)

// WebSocketHandler обрабатывает WebSocket connections
type WebSocketHandler struct {
	hub            *wsInfra.Hub
	logger         *logger.Logger
	allowedOrigins map[string]struct{}
	authConfig     middleware.AuthConfig
	upgrader       websocket.Upgrader
}

// NewWebSocketHandler создает новый handler
func NewWebSocketHandler(
	hub *wsInfra.Hub,
	allowedOrigins []string,
	authConfig middleware.AuthConfig,
	logger *logger.Logger,
) *WebSocketHandler {
	originMap := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			continue
		}
		originMap[trimmed] = struct{}{}
	}

	handler := &WebSocketHandler{
		hub:            hub,
		logger:         logger,
		allowedOrigins: originMap,
		authConfig:     authConfig,
	}

	handler.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     handler.checkOrigin,
	}

	return handler
}

func (h *WebSocketHandler) checkOrigin(r *http.Request) bool {
	if len(h.allowedOrigins) == 0 {
		return false
	}

	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return false
	}

	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}

	normalized := parsed.Scheme + "://" + parsed.Host
	if _, ok := h.allowedOrigins[normalized]; ok {
		return true
	}
	if _, ok := h.allowedOrigins["*"]; ok {
		return true
	}

	return false
}

// HandleConnection обрабатывает новое WebSocket соединение
func (h *WebSocketHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	if err := middleware.ValidateRequestAuth(r, h.authConfig); err != nil {
		h.logger.Warn("WebSocket unauthorized",
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("WebSocket upgrade failed", err)
		return
	}

	client := wsInfra.NewClient(h.hub, conn, h.logger)
	h.hub.Register(client)

	// Запускаем pumps в отдельных goroutines
	go client.WritePump()
	go client.ReadPump()
}
