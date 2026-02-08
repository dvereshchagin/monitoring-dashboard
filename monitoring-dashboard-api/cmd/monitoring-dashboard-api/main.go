package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	// Application
	applicationPort "github.com/dreschagin/monitoring-dashboard/internal/application/port"
	"github.com/dreschagin/monitoring-dashboard/internal/application/usecase"

	// Domain
	"github.com/dreschagin/monitoring-dashboard/internal/domain/service"

	// Infrastructure
	"github.com/dreschagin/monitoring-dashboard/internal/infrastructure/collector"
	natsInfra "github.com/dreschagin/monitoring-dashboard/internal/infrastructure/messaging/nats"
	wsInfra "github.com/dreschagin/monitoring-dashboard/internal/infrastructure/notification/websocket"
	"github.com/dreschagin/monitoring-dashboard/internal/infrastructure/observability/cloudwatch"
	dynamodbRepo "github.com/dreschagin/monitoring-dashboard/internal/infrastructure/persistence/dynamodb"
	"github.com/dreschagin/monitoring-dashboard/internal/infrastructure/persistence/postgres"
	s3storage "github.com/dreschagin/monitoring-dashboard/internal/infrastructure/storage/s3"

	// Interfaces
	httpInterface "github.com/dreschagin/monitoring-dashboard/internal/interfaces/http"
	"github.com/dreschagin/monitoring-dashboard/internal/interfaces/http/handler"
	"github.com/dreschagin/monitoring-dashboard/internal/interfaces/http/middleware"

	// Shared
	"github.com/dreschagin/monitoring-dashboard/pkg/config"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"

	_ "github.com/lib/pq"
)

func main() {
	// 1. Загружаем конфигурацию
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 2. Инициализируем logger
	log := logger.New(os.Getenv("LOG_LEVEL"))
	log.Info("Starting Monitoring Dashboard")

	// 3. Подключаемся к БД
	db, err := sql.Open("postgres", cfg.Database.DSN())
	if err != nil {
		log.Error("Failed to connect to database", err)
		os.Exit(1)
	}
	defer db.Close()

	// Настраиваем connection pool
	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.Database.ConnMaxIdleTime)

	// Проверяем подключение
	if err := db.Ping(); err != nil {
		log.Error("Failed to ping database", err)
		os.Exit(1)
	}
	log.Info("Database connected successfully")

	// 4. Dependency Injection - Infrastructure Layer

	// Repository
	metricRepository := postgres.NewPostgresMetricRepository(db)

	// Collectors
	metricsCollector := collector.NewSystemMetricsCollector()

	// WebSocket Hub
	hub := wsInfra.NewHub(log)

	// 5. Dependency Injection - Domain Layer

	// Domain Services
	metricAggregator := service.NewMetricAggregator()
	metricValidator := service.NewMetricValidator()

	// 5.5. CloudWatch Integration

	// CloudWatch Metrics Publisher
	var metricsPublisher applicationPort.MetricsPublisher
	if cfg.CloudWatch.MetricsEnabled {
		publisherImpl, initErr := cloudwatch.NewMetricsPublisher(context.Background(),
			cloudwatch.MetricsPublisherConfig{
				Namespace:         cfg.CloudWatch.MetricsNamespace,
				Region:            cfg.CloudWatch.Region,
				Endpoint:          cfg.CloudWatch.Endpoint,
				AccessKeyID:       cfg.CloudWatch.AccessKeyID,
				SecretAccessKey:   cfg.CloudWatch.SecretAccessKey,
				DefaultDimensions: cfg.CloudWatch.MetricsDimensions,
				BufferSize:        cfg.CloudWatch.MetricsBufferSize,
				FlushInterval:     cfg.CloudWatch.MetricsFlushInterval,
				StorageResolution: cfg.CloudWatch.MetricsStorageResolution,
			})
		if initErr != nil {
			log.Error("Failed to initialize CloudWatch metrics publisher", initErr)
			os.Exit(1)
		}
		metricsPublisher = publisherImpl
		log.Info("CloudWatch metrics publisher initialized")
	} else {
		log.Warn("CloudWatch metrics publishing is disabled")
	}

	// CloudWatch Logs Publisher
	var logsPublisher applicationPort.LogPublisher
	if cfg.CloudWatch.LogsEnabled {
		publisherImpl, initErr := cloudwatch.NewLogsPublisher(context.Background(),
			cloudwatch.LogsPublisherConfig{
				LogGroupName:    cfg.CloudWatch.LogGroupName,
				LogStreamName:   cfg.CloudWatch.LogStreamName,
				Region:          cfg.CloudWatch.Region,
				Endpoint:        cfg.CloudWatch.Endpoint,
				AccessKeyID:     cfg.CloudWatch.AccessKeyID,
				SecretAccessKey: cfg.CloudWatch.SecretAccessKey,
				BufferSize:      cfg.CloudWatch.LogsBufferSize,
				FlushInterval:   cfg.CloudWatch.LogsFlushInterval,
				AutoCreate:      true,
			})
		if initErr != nil {
			log.Error("Failed to initialize CloudWatch logs publisher", initErr)
			os.Exit(1)
		}
		logsPublisher = publisherImpl
		log.SetLogPublisher(logsPublisher)
		log.Info("CloudWatch logs publisher initialized")
	} else {
		log.Warn("CloudWatch logs publishing is disabled")
	}

	// 5.6. NATS Event Publisher
	var eventPublisher applicationPort.EventPublisher
	if cfg.NATS.Enabled {
		publisherImpl, initErr := natsInfra.NewNATSPublisher(cfg.NATS.URL, log)
		if initErr != nil {
			log.Warn("Failed to connect to NATS, continuing without event publishing", "error", initErr.Error())
		} else {
			eventPublisher = publisherImpl
			defer eventPublisher.Close()
			log.Info("NATS event publisher initialized", "url", cfg.NATS.URL)
		}
	} else {
		log.Warn("NATS event publishing is disabled")
	}

	// 6. Dependency Injection - Application Layer (Use Cases)

	collectMetricsUC := usecase.NewCollectMetricsUseCase(
		metricsCollector,
		metricRepository,
		hub,
		metricValidator,
		metricsPublisher, // Can be nil if CloudWatch disabled
		eventPublisher,   // Can be nil if NATS disabled
		log,
	)

	getCurrentMetricsUC := usecase.NewGetCurrentMetricsUseCase(
		metricRepository,
		log,
	)

	getHistoricalMetricsUC := usecase.NewGetHistoricalMetricsUseCase(
		metricRepository,
		metricAggregator,
		log,
	)

	var screenshotStorage applicationPort.ScreenshotStorage
	if cfg.S3.Enabled {
		storageImpl, initErr := s3storage.NewScreenshotStorage(context.Background(), s3storage.Config{
			Bucket:          cfg.S3.Bucket,
			Region:          cfg.S3.Region,
			Endpoint:        cfg.S3.Endpoint,
			AccessKeyID:     cfg.S3.AccessKeyID,
			SecretAccessKey: cfg.S3.SecretAccessKey,
			UsePathStyle:    cfg.S3.UsePathStyle,
			URLMode:         s3storage.URLMode(cfg.S3.URLMode),
			PresignedTTL:    cfg.S3.PresignedTTL,
		})
		if initErr != nil {
			log.Error("Failed to initialize screenshot storage", initErr)
			os.Exit(1)
		}
		screenshotStorage = storageImpl
	} else {
		log.Warn("S3 storage is disabled, screenshot uploads will fail")
	}

	var screenshotMetadataRepo applicationPort.ScreenshotMetadataRepository
	if cfg.Dynamo.Enabled {
		repoImpl, initErr := dynamodbRepo.NewScreenshotMetadataRepository(context.Background(), dynamodbRepo.Config{
			TableName:       cfg.Dynamo.TableScreenshotMetadata,
			Region:          cfg.Dynamo.Region,
			Endpoint:        cfg.Dynamo.Endpoint,
			AccessKeyID:     cfg.Dynamo.AccessKeyID,
			SecretAccessKey: cfg.Dynamo.SecretAccessKey,
			StrongReads:     cfg.Dynamo.StrongReads,
		})
		if initErr != nil {
			log.Error("Failed to initialize screenshot metadata repository", initErr)
			os.Exit(1)
		}
		screenshotMetadataRepo = repoImpl
		log.Info("Screenshot metadata repository initialized", "provider", "dynamodb")
	} else {
		log.Warn("DynamoDB screenshot metadata index is disabled, using S3 listing mode")
	}

	saveDashboardScreenshotsUC := usecase.NewSaveDashboardScreenshotsUseCase(
		screenshotStorage,
		screenshotMetadataRepo,
		usecase.SaveDashboardScreenshotsConfig{
			KeyPrefix:           cfg.S3.KeyPrefix,
			MetadataTTLDays:     cfg.Screenshot.MetadataTTLDays,
			MetadataWriteStrict: cfg.Screenshot.MetadataWriteStrict,
		},
		log,
	)
	listDashboardScreenshotsUC := usecase.NewListDashboardScreenshotsUseCase(
		screenshotStorage,
		screenshotMetadataRepo,
		usecase.ListDashboardScreenshotsConfig{
			KeyPrefix:           cfg.S3.KeyPrefix,
			FallbackToS3OnError: cfg.Screenshot.MetadataFallbackToS3,
		},
		log,
	)

	// 7. Dependency Injection - Interfaces Layer (HTTP Handlers)

	dashboardHandler := handler.NewDashboardHandler(getCurrentMetricsUC, log)
	authConfig := middleware.AuthConfig{
		Enabled:     cfg.Security.AuthEnabled,
		BearerToken: cfg.Security.AuthToken,
	}
	screenshotAuthConfig := middleware.AuthConfig{
		Enabled:     cfg.Screenshot.AuthEnabled,
		BearerToken: strings.TrimSpace(cfg.Security.AuthToken),
	}
	if screenshotAuthConfig.Enabled && screenshotAuthConfig.BearerToken == "" {
		log.Error("AUTH_BEARER_TOKEN is required when SCREENSHOT_AUTH_ENABLED=true", nil)
		os.Exit(1)
	}

	websocketHandler := handler.NewWebSocketHandler(hub, cfg.Security.AllowedOrigins, authConfig, log)
	metricsAPIHandler := handler.NewMetricsAPIHandler(getHistoricalMetricsUC, 24*time.Hour, log)
	screenshotAPIHandler := handler.NewScreenshotAPIHandler(
		saveDashboardScreenshotsUC,
		listDashboardScreenshotsUC,
		screenshotAuthConfig,
		cfg.Screenshot.MaxPayloadBytes,
		cfg.Screenshot.MaxArtifactBytes,
		cfg.Screenshot.RateLimitPerMinute,
		log,
	)
	authAPIHandler := handler.NewAuthAPIHandler(authConfig, log)
	releaseAnalyzerAPIHandler := handler.NewReleaseAnalyzerAPIHandler(
		cfg.ReleaseAnalyzer.BaseURL,
		cfg.ReleaseAnalyzer.RequestTimeout,
		log,
	)

	// Router
	router := httpInterface.NewRouter(
		dashboardHandler,
		websocketHandler,
		metricsAPIHandler,
		screenshotAPIHandler,
		authAPIHandler,
		releaseAnalyzerAPIHandler,
		cfg.Security,
		log,
	)

	// 8. Запускаем фоновые процессы

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Запускаем WebSocket hub
	go hub.Run()
	log.Info("WebSocket hub started")

	// Запускаем сборщик метрик (каждые 2 секунды)
	go func() {
		ticker := time.NewTicker(cfg.Metrics.CollectionInterval)
		defer ticker.Stop()

		log.Info("Metrics collector started",
			"interval", cfg.Metrics.CollectionInterval.String())

		for {
			select {
			case <-ticker.C:
				if err := collectMetricsUC.Execute(ctx); err != nil {
					log.Error("Failed to collect metrics", err)
				}
			case <-ctx.Done():
				log.Info("Metrics collector stopped")
				return
			}
		}
	}()

	// 9. Настраиваем HTTP сервер

	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router.Setup(),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Канал для получения сигналов ОС
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Запускаем сервер в отдельной goroutine
	go func() {
		log.Info("HTTP server starting", "port", cfg.Server.Port)
		log.Info("Dashboard available at http://localhost:" + cfg.Server.Port)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server failed", err)
			os.Exit(1)
		}
	}()

	// 10. Ожидаем сигнал для graceful shutdown

	<-sigChan
	log.Info("Shutdown signal received, starting graceful shutdown...")

	// Останавливаем сборщик метрик
	cancel()

	// Даем время на завершение текущих операций
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	// Flush CloudWatch buffers before shutdown
	if metricsPublisher != nil {
		log.Info("Flushing CloudWatch metrics buffer...")
		if err := metricsPublisher.Flush(shutdownCtx); err != nil {
			log.Error("Failed to flush CloudWatch metrics", err)
		}
	}

	if logsPublisher != nil {
		log.Info("Flushing CloudWatch logs buffer...")
		if err := logsPublisher.Flush(shutdownCtx); err != nil {
			log.Error("Failed to flush CloudWatch logs", err)
		}
	}

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("Server shutdown error", err)
	}

	log.Info("Server stopped gracefully")
}
