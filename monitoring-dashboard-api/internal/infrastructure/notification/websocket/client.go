package websocket

import (
	"time"

	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
	"github.com/gorilla/websocket"
)

const (
	// Время ожидания для write операций
	writeWait = 10 * time.Second

	// Время ожидания pong от клиента
	pongWait = 60 * time.Second

	// Интервал ping сообщений (должен быть меньше pongWait)
	pingPeriod = 54 * time.Second

	// Максимальный размер сообщения
	maxMessageSize = 512
)

// Client представляет WebSocket клиента
type Client struct {
	// WebSocket connection
	conn *websocket.Conn

	// Hub к которому принадлежит клиент
	hub *Hub

	// Канал для отправки сообщений
	send chan Message

	// Logger
	logger *logger.Logger
}

// NewClient создает нового WebSocket клиента
func NewClient(hub *Hub, conn *websocket.Conn, logger *logger.Logger) *Client {
	return &Client{
		conn:   conn,
		hub:    hub,
		send:   make(chan Message, 256),
		logger: logger,
	}
}

// ReadPump читает сообщения от клиента
// Запускается в отдельной goroutine
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		if err := c.conn.Close(); err != nil {
			c.logger.Error("WebSocket close error", err)
		}
	}()

	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		c.logger.Error("WebSocket set read deadline error", err)
		return
	}
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		// Читаем сообщения от клиента (обычно pong responses)
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Error("WebSocket read error", err)
			}
			break
		}
	}
}

// WritePump отправляет сообщения клиенту
// Запускается в отдельной goroutine
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		if err := c.conn.Close(); err != nil {
			c.logger.Error("WebSocket close error", err)
		}
	}()

	for {
		select {
		case message, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				c.logger.Error("WebSocket set write deadline error", err)
				return
			}
			if !ok {
				// Hub закрыл канал
				if err := c.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					c.logger.Error("WebSocket close message error", err)
				}
				return
			}

			// Отправляем JSON сообщение
			if err := c.conn.WriteJSON(message); err != nil {
				c.logger.Error("WebSocket write error", err)
				return
			}

		case <-ticker.C:
			// Отправляем ping
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				c.logger.Error("WebSocket set write deadline error", err)
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
