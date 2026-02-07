package port

import "github.com/dreschagin/monitoring-dashboard/internal/application/dto"

// NotificationService определяет интерфейс для отправки уведомлений (Port)
// Реализация будет в Infrastructure слое (WebSocket Hub)
type NotificationService interface {
	// Broadcast отправляет snapshot метрик всем подключенным клиентам
	Broadcast(snapshot *dto.MetricSnapshotDTO)

	// BroadcastAlert отправляет alert всем подключенным клиентам
	BroadcastAlert(alert *dto.AlertDTO)

	// ClientCount возвращает количество подключенных клиентов
	ClientCount() int
}
