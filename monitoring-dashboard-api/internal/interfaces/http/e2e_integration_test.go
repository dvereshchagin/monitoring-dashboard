//go:build integration
// +build integration

package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dreschagin/monitoring-dashboard/internal/application/dto"
	"github.com/dreschagin/monitoring-dashboard/internal/application/usecase"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/entity"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/service"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/valueobject"
	wsInfra "github.com/dreschagin/monitoring-dashboard/internal/infrastructure/notification/websocket"
	dynamodbRepo "github.com/dreschagin/monitoring-dashboard/internal/infrastructure/persistence/dynamodb"
	"github.com/dreschagin/monitoring-dashboard/internal/infrastructure/persistence/postgres"
	s3storage "github.com/dreschagin/monitoring-dashboard/internal/infrastructure/storage/s3"
	"github.com/dreschagin/monitoring-dashboard/internal/interfaces/http/handler"
	"github.com/dreschagin/monitoring-dashboard/internal/interfaces/http/middleware"
	"github.com/dreschagin/monitoring-dashboard/pkg/config"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
	_ "github.com/lib/pq"
)

const (
	integrationToken = "integration-token"
)

type integrationEnv struct {
	postgresDSN     string
	s3Endpoint      string
	s3Region        string
	s3AccessKey     string
	s3SecretKey     string
	s3Bucket        string
	s3UsePathStyle  bool
	dynamoEndpoint  string
	dynamoRegion    string
	dynamoAccessKey string
	dynamoSecretKey string
	dynamoTable     string
}

func loadIntegrationEnv() integrationEnv {
	return integrationEnv{
		postgresDSN:     getenv("INTEGRATION_POSTGRES_DSN", "host=localhost port=5432 user=postgres password=postgres dbname=monitoring sslmode=disable"),
		s3Endpoint:      getenv("INTEGRATION_S3_ENDPOINT", "http://localhost:9000"),
		s3Region:        getenv("INTEGRATION_S3_REGION", "us-east-1"),
		s3AccessKey:     getenv("INTEGRATION_S3_ACCESS_KEY", "minioadmin"),
		s3SecretKey:     getenv("INTEGRATION_S3_SECRET_KEY", "minioadmin"),
		s3Bucket:        getenv("INTEGRATION_S3_BUCKET", "dashboard-screenshots-e2e"),
		s3UsePathStyle:  true,
		dynamoEndpoint:  getenv("INTEGRATION_DYNAMO_ENDPOINT", "http://localhost:8000"),
		dynamoRegion:    getenv("INTEGRATION_DYNAMO_REGION", "us-east-1"),
		dynamoAccessKey: getenv("INTEGRATION_DYNAMO_ACCESS_KEY", "dynamo"),
		dynamoSecretKey: getenv("INTEGRATION_DYNAMO_SECRET_KEY", "dynamo"),
		dynamoTable:     getenv("INTEGRATION_DYNAMO_TABLE", "dashboard_screenshot_metadata_e2e"),
	}
}

func TestE2EIntegrationMetricsHistory(t *testing.T) {
	env := loadIntegrationEnv()
	ctx := context.Background()

	db := connectPostgres(t, env.postgresDSN)
	t.Cleanup(func() { _ = db.Close() })
	applyMigrations(t, db)
	cleanupMetrics(t, db)

	repo := postgres.NewPostgresMetricRepository(db)
	seedMetricsIntegration(t, repo)

	server := integrationServer(t, repo, env)
	client := server.Client()

	resp := doRequest(t, client, http.MethodGet, server.URL+"/api/v1/metrics/history?type=cpu&duration=1h", nil, map[string]string{
		"Authorization": "Bearer " + integrationToken,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for metrics history, got %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	var history dto.MetricHistoryDTO
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		t.Fatalf("decode history response: %v", err)
	}
	if history.Type != "cpu" || len(history.Metrics) == 0 {
		t.Fatalf("expected cpu history, got type=%s len=%d", history.Type, len(history.Metrics))
	}

	healthResp := doRequest(t, client, http.MethodGet, server.URL+"/healthz", nil, nil)
	if healthResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for healthz, got %d", healthResp.StatusCode)
	}
	healthResp.Body.Close()

	readyResp := doRequest(t, client, http.MethodGet, server.URL+"/readyz", nil, nil)
	if readyResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for readyz, got %d", readyResp.StatusCode)
	}
	readyResp.Body.Close()

	_ = ctx
}

func TestE2EIntegrationScreenshots(t *testing.T) {
	env := loadIntegrationEnv()
	ctx := context.Background()

	db := connectPostgres(t, env.postgresDSN)
	t.Cleanup(func() { _ = db.Close() })
	applyMigrations(t, db)

	repo := postgres.NewPostgresMetricRepository(db)
	seedMetricsIntegration(t, repo)

	ensureS3Bucket(t, ctx, env)
	ensureDynamoTable(t, ctx, env)

	server := integrationServer(t, repo, env)
	client := server.Client()

	reqBody := buildIntegrationScreenshotRequest(t)
	saveResp := doRequest(t, client, http.MethodPost, server.URL+"/api/v1/screenshots/dashboard", reqBody, map[string]string{
		"Authorization": "Bearer " + integrationToken,
		"Content-Type":  "application/json",
	})
	if saveResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for screenshot save, got %d", saveResp.StatusCode)
	}
	saveResp.Body.Close()

	listResp := doRequest(t, client, http.MethodGet, server.URL+"/api/v1/screenshots/dashboard?dashboard_id=main&limit=10", nil, map[string]string{
		"Authorization": "Bearer " + integrationToken,
	})
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for screenshot list, got %d", listResp.StatusCode)
	}
	defer listResp.Body.Close()

	var listPayload struct {
		Items []struct {
			Type string `json:"type"`
		} `json:"items"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode screenshot list response: %v", err)
	}
	if len(listPayload.Items) == 0 {
		t.Fatal("expected screenshot items from dynamodb")
	}
}

func integrationServer(t *testing.T, repo *postgres.PostgresMetricRepository, env integrationEnv) *httptest.Server {
	t.Helper()
	log := logger.New("error")

	aggregator := service.NewMetricAggregator()
	getHistoricalMetricsUC := usecase.NewGetHistoricalMetricsUseCase(repo, aggregator, log)
	getCurrentMetricsUC := usecase.NewGetCurrentMetricsUseCase(repo, log)

	hub := wsInfra.NewHub(log)
	websocketHandler := handler.NewWebSocketHandler(hub, []string{"http://localhost:8080"}, middleware.AuthConfig{
		Enabled:     true,
		BearerToken: integrationToken,
	}, log)

	dashboardHandler := handler.NewDashboardHandler(getCurrentMetricsUC, log)
	metricsAPIHandler := handler.NewMetricsAPIHandler(getHistoricalMetricsUC, 24*time.Hour, log)

	s3Store := buildS3Storage(t, env)
	metadataRepo := buildDynamoRepo(t, env)
	saveScreenshotsUC := usecase.NewSaveDashboardScreenshotsUseCase(s3Store, metadataRepo, usecase.SaveDashboardScreenshotsConfig{}, log)
	listScreenshotsUC := usecase.NewListDashboardScreenshotsUseCase(s3Store, metadataRepo, usecase.ListDashboardScreenshotsConfig{}, log)
	screenshotAPIHandler := handler.NewScreenshotAPIHandler(
		saveScreenshotsUC,
		listScreenshotsUC,
		middleware.AuthConfig{Enabled: true, BearerToken: integrationToken},
		5*1024*1024,
		1*1024*1024,
		100,
		log,
	)

	authAPIHandler := handler.NewAuthAPIHandler(middleware.AuthConfig{Enabled: true, BearerToken: integrationToken}, log)
	releaseAnalyzerAPIHandler := handler.NewReleaseAnalyzerAPIHandler("http://example.invalid", 2*time.Second, log)

	router := NewRouter(
		dashboardHandler,
		websocketHandler,
		metricsAPIHandler,
		screenshotAPIHandler,
		authAPIHandler,
		releaseAnalyzerAPIHandler,
		config.SecurityConfig{
			AllowedOrigins: []string{"http://localhost:8080"},
			AuthEnabled:    true,
			AuthToken:      integrationToken,
		},
		log,
	)

	server := httptest.NewServer(router.Setup())
	t.Cleanup(server.Close)
	return server
}

func connectPostgres(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}
	return db
}

func applyMigrations(t *testing.T, db *sql.DB) {
	t.Helper()
	paths := []string{
		filepath.Join("internal", "infrastructure", "persistence", "postgres", "migrations", "001_init.sql"),
		filepath.Join("internal", "infrastructure", "persistence", "postgres", "migrations", "002_indexes.sql"),
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", path, err)
		}
		sqlText := stripGooseDirectives(string(raw))
		if strings.TrimSpace(sqlText) == "" {
			continue
		}
		if _, err := db.Exec(sqlText); err != nil {
			t.Fatalf("apply migration %s: %v", path, err)
		}
	}
}

func stripGooseDirectives(raw string) string {
	lines := strings.Split(raw, "\n")
	filtered := lines[:0]
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-- +goose") {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

func cleanupMetrics(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("DELETE FROM metrics"); err != nil {
		t.Fatalf("cleanup metrics: %v", err)
	}
}

func seedMetricsIntegration(t *testing.T, repo *postgres.PostgresMetricRepository) {
	t.Helper()
	now := time.Now().UTC()
	entries := []struct {
		id         string
		metricType valueobject.MetricType
		value      float64
		unit       string
		collected  time.Time
	}{
		{"cpu-1", valueobject.CPU, 40, "%", now.Add(-30 * time.Minute)},
		{"cpu-2", valueobject.CPU, 55, "%", now.Add(-5 * time.Minute)},
		{"memory-1", valueobject.Memory, 60, "%", now.Add(-5 * time.Minute)},
		{"disk-1", valueobject.Disk, 70, "%", now.Add(-5 * time.Minute)},
		{"network-1", valueobject.Network, 80, "MB/s", now.Add(-5 * time.Minute)},
	}

	for _, entry := range entries {
		value, err := valueobject.NewMetricValue(entry.value, entry.unit)
		if err != nil {
			t.Fatalf("metric value: %v", err)
		}
		metric := entity.Reconstruct(entry.id, entry.metricType, entry.metricType.String(), value, nil, entry.collected, entry.collected)
		if err := repo.Save(context.Background(), metric); err != nil {
			t.Fatalf("seed metrics: %v", err)
		}
	}
}

func buildS3Storage(t *testing.T, env integrationEnv) *s3storage.ScreenshotStorage {
	t.Helper()
	store, err := s3storage.NewScreenshotStorage(context.Background(), s3storage.Config{
		Bucket:          env.s3Bucket,
		Region:          env.s3Region,
		Endpoint:        env.s3Endpoint,
		AccessKeyID:     env.s3AccessKey,
		SecretAccessKey: env.s3SecretKey,
		UsePathStyle:    env.s3UsePathStyle,
		URLMode:         s3storage.URLModePresigned,
		PresignedTTL:    2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("init s3 storage: %v", err)
	}
	return store
}

func buildDynamoRepo(t *testing.T, env integrationEnv) *dynamodbRepo.ScreenshotMetadataRepository {
	t.Helper()
	repo, err := dynamodbRepo.NewScreenshotMetadataRepository(context.Background(), dynamodbRepo.Config{
		TableName:       env.dynamoTable,
		Region:          env.dynamoRegion,
		Endpoint:        env.dynamoEndpoint,
		AccessKeyID:     env.dynamoAccessKey,
		SecretAccessKey: env.dynamoSecretKey,
		StrongReads:     true,
	})
	if err != nil {
		t.Fatalf("init dynamodb repo: %v", err)
	}
	return repo
}

func ensureS3Bucket(t *testing.T, ctx context.Context, env integrationEnv) {
	t.Helper()
	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(env.s3Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			env.s3AccessKey,
			env.s3SecretKey,
			"",
		)),
	)
	if err != nil {
		t.Fatalf("load aws config: %v", err)
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = &env.s3Endpoint
		options.UsePathStyle = env.s3UsePathStyle
	})

	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: &env.s3Bucket,
	})
	if err != nil && !isBucketExistsError(err) {
		t.Fatalf("create bucket: %v", err)
	}
}

func isBucketExistsError(err error) bool {
	var alreadyOwned *s3.BucketAlreadyOwnedByYou
	var alreadyExists *s3.BucketAlreadyExists
	if errors.As(err, &alreadyOwned) || errors.As(err, &alreadyExists) {
		return true
	}
	if strings.Contains(err.Error(), "BucketAlreadyOwnedByYou") || strings.Contains(err.Error(), "BucketAlreadyExists") {
		return true
	}
	return false
}

func ensureDynamoTable(t *testing.T, ctx context.Context, env integrationEnv) {
	t.Helper()
	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(env.dynamoRegion),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			env.dynamoAccessKey,
			env.dynamoSecretKey,
			"",
		)),
	)
	if err != nil {
		t.Fatalf("load dynamo config: %v", err)
	}
	client := dynamodb.NewFromConfig(awsCfg, func(options *dynamodb.Options) {
		options.BaseEndpoint = &env.dynamoEndpoint
	})

	_, err = client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: &env.dynamoTable,
	})
	if err == nil {
		return
	}

	_, err = client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: &env.dynamoTable,
		AttributeDefinitions: []ddbtypes.AttributeDefinition{
			{AttributeName: stringPtr("PK"), AttributeType: ddbtypes.ScalarAttributeTypeS},
			{AttributeName: stringPtr("SK"), AttributeType: ddbtypes.ScalarAttributeTypeS},
			{AttributeName: stringPtr("GSI1PK"), AttributeType: ddbtypes.ScalarAttributeTypeS},
			{AttributeName: stringPtr("GSI1SK"), AttributeType: ddbtypes.ScalarAttributeTypeS},
		},
		KeySchema: []ddbtypes.KeySchemaElement{
			{AttributeName: stringPtr("PK"), KeyType: ddbtypes.KeyTypeHash},
			{AttributeName: stringPtr("SK"), KeyType: ddbtypes.KeyTypeRange},
		},
		BillingMode: ddbtypes.BillingModePayPerRequest,
		GlobalSecondaryIndexes: []ddbtypes.GlobalSecondaryIndex{
			{
				IndexName: stringPtr("GSI1"),
				KeySchema: []ddbtypes.KeySchemaElement{
					{AttributeName: stringPtr("GSI1PK"), KeyType: ddbtypes.KeyTypeHash},
					{AttributeName: stringPtr("GSI1SK"), KeyType: ddbtypes.KeyTypeRange},
				},
				Projection: &ddbtypes.Projection{ProjectionType: ddbtypes.ProjectionTypeAll},
			},
		},
	})
	if err != nil {
		t.Fatalf("create dynamodb table: %v", err)
	}

	waiter := dynamodb.NewTableExistsWaiter(client)
	if err := waiter.Wait(ctx, &dynamodb.DescribeTableInput{TableName: &env.dynamoTable}, 30*time.Second); err != nil {
		t.Fatalf("wait for table: %v", err)
	}
}

func buildIntegrationScreenshotRequest(t *testing.T) *bytes.Buffer {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(minimalPngBase64)
	if err != nil {
		t.Fatalf("decode base64 test png: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)

	type artifact struct {
		Type        string `json:"type"`
		ContentType string `json:"content_type"`
		DataBase64  string `json:"data_base64"`
	}
	payload := struct {
		DashboardID string     `json:"dashboard_id"`
		CapturedAt  time.Time  `json:"captured_at"`
		Artifacts   []artifact `json:"artifacts"`
	}{
		DashboardID: "main",
		CapturedAt:  time.Now().UTC(),
		Artifacts: []artifact{
			{Type: "cpu_card", ContentType: "image/png", DataBase64: encoded},
			{Type: "memory_card", ContentType: "image/png", DataBase64: encoded},
			{Type: "disk_card", ContentType: "image/png", DataBase64: encoded},
			{Type: "network_card", ContentType: "image/png", DataBase64: encoded},
			{Type: "cpu_chart", ContentType: "image/png", DataBase64: encoded},
			{Type: "memory_chart", ContentType: "image/png", DataBase64: encoded},
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal screenshot request: %v", err)
	}
	return bytes.NewBuffer(raw)
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func stringPtr(value string) *string {
	return &value
}
