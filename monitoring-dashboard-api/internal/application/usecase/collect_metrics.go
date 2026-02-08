package usecase

import (
	"context"
	"fmt"

	"github.com/dreschagin/monitoring-dashboard/internal/application/dto"
	"github.com/dreschagin/monitoring-dashboard/internal/application/port"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/entity"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/repository"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/service"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/valueobject"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
)

// CollectMetricsUseCase координирует сбор, валидацию, сохранение и рассылку метрик
type CollectMetricsUseCase struct {
	collector        port.MetricsCollector
	repository       repository.MetricRepository
	notifier         port.NotificationService
	validator        *service.MetricValidator
	metricsPublisher port.MetricsPublisher // Optional CloudWatch publisher
	eventPublisher   port.EventPublisher   // Optional NATS event publisher
	logger           *logger.Logger
}

// NewCollectMetricsUseCase создает новый use case
func NewCollectMetricsUseCase(
	collector port.MetricsCollector,
	repository repository.MetricRepository,
	notifier port.NotificationService,
	validator *service.MetricValidator,
	metricsPublisher port.MetricsPublisher, // Can be nil if CloudWatch disabled
	eventPublisher port.EventPublisher,     // Can be nil if NATS disabled
	logger *logger.Logger,
) *CollectMetricsUseCase {
	return &CollectMetricsUseCase{
		collector:        collector,
		repository:       repository,
		notifier:         notifier,
		validator:        validator,
		metricsPublisher: metricsPublisher,
		eventPublisher:   eventPublisher,
		logger:           logger,
	}
}

// Execute выполняет сбор метрик
func (uc *CollectMetricsUseCase) Execute(ctx context.Context) error {
	// 1. Собираем сырые метрики от collector
	uc.logger.Debug("Collecting metrics from system")
	rawMetrics, err := uc.collector.CollectAll(ctx)
	if err != nil {
		uc.logger.Error("Failed to collect metrics", err)
		return fmt.Errorf("failed to collect metrics: %w", err)
	}

	uc.logger.Debug("Collected raw metrics", "count", len(rawMetrics))

	// 2. Конвертируем в Domain Entities
	metrics := make([]*entity.Metric, 0, len(rawMetrics))
	for _, raw := range rawMetrics {
		metric, err := entity.NewMetric(raw.Type, raw.Name, raw.Value)
		if err != nil {
			uc.logger.Warn("Skipping invalid metric", "type", raw.Type, "name", raw.Name, "error", err.Error())
			continue
		}

		// Добавляем метаданные
		if raw.Metadata != nil {
			for key, value := range raw.Metadata {
				metric.SetMetadata(key, value)
			}
		}

		// Валидация метрики
		if err := uc.validator.Validate(metric); err != nil {
			uc.logger.Warn("Metric validation failed", "id", metric.ID(), "error", err.Error())
			continue
		}

		// Проверка на разумность значений
		if !uc.validator.IsReasonable(metric) {
			uc.logger.Warn("Metric value is unreasonable", "id", metric.ID(), "value", metric.Value().Raw())
			continue
		}

		metrics = append(metrics, metric)
	}

	if len(metrics) == 0 {
		uc.logger.Warn("No valid metrics to save")
		return nil
	}

	uc.logger.Debug("Converted to domain entities", "valid_count", len(metrics))

	// 3. Сохраняем в репозитории (batch insert)
	if err := uc.repository.SaveBatch(ctx, metrics); err != nil {
		uc.logger.Error("Failed to save metrics batch", err)
		return fmt.Errorf("failed to save metrics: %w", err)
	}

	uc.logger.Debug("Metrics saved to repository", "count", len(metrics))

	// 3.5. Публикуем в CloudWatch если включено
	if uc.metricsPublisher != nil {
		if err := uc.metricsPublisher.PublishBatch(ctx, metrics); err != nil {
			// Log error but don't fail the entire operation (graceful degradation)
			uc.logger.Error("Failed to publish metrics to CloudWatch", err)
		} else {
			uc.logger.Debug("Metrics published to CloudWatch", "count", len(metrics))
		}
	}

	// 4. Создаем snapshot для рассылки
	metricsMap := uc.buildMetricsMap(metrics)
	snapshot := dto.NewMetricSnapshotDTO(metricsMap)

	// 5. Рассылаем через WebSocket
	uc.notifier.Broadcast(snapshot)
	uc.logger.Debug("Metrics broadcasted to clients", "client_count", uc.notifier.ClientCount())

	// 5.5. Публикуем событие в NATS если включено
	if uc.eventPublisher != nil {
		event := map[string]interface{}{
			"event_type":     "metric.collected",
			"aggregate_id":   fmt.Sprintf("metrics-batch-%d", snapshot.Timestamp.Unix()),
			"aggregate_type": "metrics",
			"payload": map[string]interface{}{
				"metrics_count": len(metrics),
				"collected_at":  snapshot.Timestamp,
				"cpu_usage":     snapshot.CPU,
				"memory_usage":  snapshot.Memory,
				"disk_usage":    snapshot.Disk,
			},
			"version": 1,
		}

		if err := uc.eventPublisher.PublishEvent(ctx, "events.metrics.collected", event); err != nil {
			// Log error but don't fail the entire operation (graceful degradation)
			uc.logger.Error("Failed to publish metrics event to NATS", err)
		} else {
			uc.logger.Debug("Metrics event published to NATS")
		}
	}

	// 6. Отправляем alerts для критических метрик
	uc.checkAndSendAlerts(ctx, metrics)

	return nil
}

// buildMetricsMap строит map метрик по типам (берем последнюю метрику каждого типа)
func (uc *CollectMetricsUseCase) buildMetricsMap(metrics []*entity.Metric) map[valueobject.MetricType]*entity.Metric {
	metricsMap := make(map[valueobject.MetricType]*entity.Metric)

	for _, metric := range metrics {
		// Перезаписываем если уже есть (оставляем последнюю)
		metricsMap[metric.Type()] = metric
	}

	return metricsMap
}

// checkAndSendAlerts проверяет критические метрики и отправляет alerts
func (uc *CollectMetricsUseCase) checkAndSendAlerts(ctx context.Context, metrics []*entity.Metric) {
	for _, metric := range metrics {
		if metric.IsCritical() {
			message := fmt.Sprintf("%s usage is critical: %.2f%s",
				metric.Type().String(),
				metric.Value().Raw(),
				metric.Value().Unit())

			alert := dto.NewAlertDTO(metric, message)
			uc.notifier.BroadcastAlert(alert)
			uc.logger.Warn("Critical metric detected", "type", metric.Type(), "value", metric.Value().Raw())

			// Публикуем событие критической метрики в NATS
			if uc.eventPublisher != nil {
				event := map[string]interface{}{
					"event_type":     "metric.critical",
					"aggregate_id":   metric.ID(),
					"aggregate_type": "metric",
					"payload": map[string]interface{}{
						"metric_type": metric.Type().String(),
						"value":       metric.Value().Raw(),
						"unit":        metric.Value().Unit(),
						"message":     message,
					},
					"version": 1,
				}

				if err := uc.eventPublisher.PublishEvent(ctx, "events.metrics.critical", event); err != nil {
					uc.logger.Error("Failed to publish critical metric event", err)
				}
			}
		}
	}
}
