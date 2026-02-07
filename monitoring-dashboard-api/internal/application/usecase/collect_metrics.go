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
	collector  port.MetricsCollector
	repository repository.MetricRepository
	notifier   port.NotificationService
	validator  *service.MetricValidator
	logger     *logger.Logger
}

// NewCollectMetricsUseCase создает новый use case
func NewCollectMetricsUseCase(
	collector port.MetricsCollector,
	repository repository.MetricRepository,
	notifier port.NotificationService,
	validator *service.MetricValidator,
	logger *logger.Logger,
) *CollectMetricsUseCase {
	return &CollectMetricsUseCase{
		collector:  collector,
		repository: repository,
		notifier:   notifier,
		validator:  validator,
		logger:     logger,
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

	// 4. Создаем snapshot для рассылки
	metricsMap := uc.buildMetricsMap(metrics)
	snapshot := dto.NewMetricSnapshotDTO(metricsMap)

	// 5. Рассылаем через WebSocket
	uc.notifier.Broadcast(snapshot)
	uc.logger.Debug("Metrics broadcasted to clients", "client_count", uc.notifier.ClientCount())

	// 6. Отправляем alerts для критических метрик
	uc.checkAndSendAlerts(metrics)

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
func (uc *CollectMetricsUseCase) checkAndSendAlerts(metrics []*entity.Metric) {
	for _, metric := range metrics {
		if metric.IsCritical() {
			message := fmt.Sprintf("%s usage is critical: %.2f%s",
				metric.Type().String(),
				metric.Value().Raw(),
				metric.Value().Unit())

			alert := dto.NewAlertDTO(metric, message)
			uc.notifier.BroadcastAlert(alert)
			uc.logger.Warn("Critical metric detected", "type", metric.Type(), "value", metric.Value().Raw())
		}
	}
}
