package websocket

import (
	"sync"

	"github.com/dreschagin/monitoring-dashboard/internal/application/dto"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
)

// Hub управляет WebSocket клиентами и рассылает сообщения
// Реализует интерфейс port.NotificationService
type Hub struct {
	// Зарегистрированные клиенты
	clients map[*Client]bool

	// Канал для broadcast сообщений
	broadcast chan *dto.MetricSnapshotDTO

	// Канал для broadcast alerts
	broadcastAlert chan *dto.AlertDTO

	// Канал для регистрации клиентов
	register chan *Client

	// Канал для удаления клиентов
	unregister chan *Client

	// Mutex для защиты clients map
	mu sync.RWMutex

	// Logger
	logger *logger.Logger
}

// NewHub создает новый WebSocket hub
func NewHub(logger *logger.Logger) *Hub {
	return &Hub{
		clients:        make(map[*Client]bool),
		broadcast:      make(chan *dto.MetricSnapshotDTO, 256),
		broadcastAlert: make(chan *dto.AlertDTO, 256),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		logger:         logger,
	}
}

// Run запускает hub (должен быть запущен в отдельной goroutine)
func (h *Hub) Run() {
	h.logger.Info("WebSocket hub started")

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.logger.Debug("Client registered", "total_clients", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			h.logger.Debug("Client unregistered", "total_clients", len(h.clients))

		case snapshot := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- Message{Type: "snapshot", Data: snapshot}:
					// Сообщение отправлено
				default:
					// Канал клиента заполнен, закрываем соединение
					close(client.send)
					delete(h.clients, client)
					h.logger.Warn("Client channel full, disconnected")
				}
			}
			h.mu.RUnlock()

		case alert := <-h.broadcastAlert:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- Message{Type: "alert", Data: alert}:
					// Alert отправлен
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
			h.logger.Debug("Alert broadcasted to clients", "level", alert.Level)
		}
	}
}

// Register регистрирует нового клиента
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister удаляет клиента
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Broadcast отправляет snapshot всем клиентам (реализация port.NotificationService)
func (h *Hub) Broadcast(snapshot *dto.MetricSnapshotDTO) {
	select {
	case h.broadcast <- snapshot:
		// Snapshot отправлен в канал
	default:
		h.logger.Warn("Broadcast channel full, dropping snapshot")
	}
}

// BroadcastAlert отправляет alert всем клиентам (реализация port.NotificationService)
func (h *Hub) BroadcastAlert(alert *dto.AlertDTO) {
	select {
	case h.broadcastAlert <- alert:
		// Alert отправлен в канал
	default:
		h.logger.Warn("Broadcast alert channel full, dropping alert")
	}
}

// ClientCount возвращает количество подключенных клиентов (реализация port.NotificationService)
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Message представляет сообщение для отправки клиенту
type Message struct {
	Type string      `json:"type"` // "snapshot" или "alert"
	Data interface{} `json:"data"`
}
