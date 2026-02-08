package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dreschagin/monitoring-dashboard/internal/application/dto"
	"github.com/dreschagin/monitoring-dashboard/internal/application/port"
	"github.com/dreschagin/monitoring-dashboard/internal/application/usecase"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/entity"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/service"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/valueobject"
	wsInfra "github.com/dreschagin/monitoring-dashboard/internal/infrastructure/notification/websocket"
	"github.com/dreschagin/monitoring-dashboard/internal/interfaces/http/handler"
	"github.com/dreschagin/monitoring-dashboard/internal/interfaces/http/middleware"
	"github.com/dreschagin/monitoring-dashboard/pkg/config"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
)

const (
	testToken        = "test-token"
	minimalPngBase64 = "iVBORw0KGgo=" // PNG signature only
)

type memoryMetricRepo struct {
	mu      sync.RWMutex
	metrics []*entity.Metric
}

func newMemoryMetricRepo() *memoryMetricRepo {
	return &memoryMetricRepo{
		metrics: make([]*entity.Metric, 0),
	}
}

func (r *memoryMetricRepo) Save(_ context.Context, metric *entity.Metric) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics = append(r.metrics, metric)
	return nil
}

func (r *memoryMetricRepo) SaveBatch(_ context.Context, metrics []*entity.Metric) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics = append(r.metrics, metrics...)
	return nil
}

func (r *memoryMetricRepo) FindByID(_ context.Context, id string) (*entity.Metric, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, metric := range r.metrics {
		if metric.ID() == id {
			return metric, nil
		}
	}
	return nil, nil
}

func (r *memoryMetricRepo) FindByType(_ context.Context, metricType valueobject.MetricType, limit int) ([]*entity.Metric, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*entity.Metric, 0)
	for _, metric := range r.metrics {
		if metric.Type() != metricType {
			continue
		}
		result = append(result, metric)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (r *memoryMetricRepo) FindByTimeRange(_ context.Context, metricType valueobject.MetricType, timeRange valueobject.TimeRange) ([]*entity.Metric, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*entity.Metric, 0)
	for _, metric := range r.metrics {
		if metric.Type() != metricType {
			continue
		}
		if timeRange.Contains(metric.CollectedAt()) {
			result = append(result, metric)
		}
	}
	return result, nil
}

func (r *memoryMetricRepo) FindLatest(_ context.Context) (map[valueobject.MetricType]*entity.Metric, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	latest := make(map[valueobject.MetricType]*entity.Metric)
	for _, metric := range r.metrics {
		current, ok := latest[metric.Type()]
		if !ok || metric.CollectedAt().After(current.CollectedAt()) {
			latest[metric.Type()] = metric
		}
	}
	return latest, nil
}

func (r *memoryMetricRepo) FindLatestByType(_ context.Context, metricType valueobject.MetricType) (*entity.Metric, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var latest *entity.Metric
	for _, metric := range r.metrics {
		if metric.Type() != metricType {
			continue
		}
		if latest == nil || metric.CollectedAt().After(latest.CollectedAt()) {
			latest = metric
		}
	}
	return latest, nil
}

func (r *memoryMetricRepo) DeleteOlderThan(_ context.Context, age valueobject.TimeRange) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	threshold := time.Now().Add(-age.Duration())
	filtered := r.metrics[:0]
	for _, metric := range r.metrics {
		if metric.CollectedAt().After(threshold) {
			filtered = append(filtered, metric)
		}
	}
	r.metrics = filtered
	return nil
}

func (r *memoryMetricRepo) Count(_ context.Context, metricType valueobject.MetricType) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var count int64
	for _, metric := range r.metrics {
		if metric.Type() == metricType {
			count++
		}
	}
	return count, nil
}

type memoryScreenshotStorage struct {
	mu      sync.RWMutex
	objects map[string]storedScreenshot
}

type storedScreenshot struct {
	contentType  string
	data         []byte
	lastModified time.Time
}

func newMemoryScreenshotStorage() *memoryScreenshotStorage {
	return &memoryScreenshotStorage{
		objects: make(map[string]storedScreenshot),
	}
}

func (s *memoryScreenshotStorage) PutObject(_ context.Context, key, contentType string, body []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = storedScreenshot{
		contentType:  contentType,
		data:         append([]byte(nil), body...),
		lastModified: time.Now().UTC(),
	}
	return "https://storage.local/" + key, nil
}

func (s *memoryScreenshotStorage) ListObjects(_ context.Context, prefix string, limit int) ([]port.ScreenshotObject, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]port.ScreenshotObject, 0, len(s.objects))
	for key, obj := range s.objects {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		items = append(items, port.ScreenshotObject{
			Key:          key,
			LastModified: obj.lastModified,
			URL:          "https://storage.local/" + key,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].LastModified.After(items[j].LastModified)
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (s *memoryScreenshotStorage) GetObjectURL(_ context.Context, key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.objects[key]; !ok {
		return "", http.ErrMissingFile
	}
	return "https://storage.local/" + key, nil
}

func newTestServer(t *testing.T, releaseAnalyzerBaseURL string) (*httptest.Server, *memoryScreenshotStorage) {
	t.Helper()

	log := logger.New("error")
	repo := newMemoryMetricRepo()
	seedMetrics(t, repo)

	aggregator := service.NewMetricAggregator()
	getHistoricalMetricsUC := usecase.NewGetHistoricalMetricsUseCase(repo, aggregator, log)
	getCurrentMetricsUC := usecase.NewGetCurrentMetricsUseCase(repo, log)

	hub := wsInfra.NewHub(log)
	websocketHandler := handler.NewWebSocketHandler(hub, []string{"http://localhost:8080"}, middleware.AuthConfig{
		Enabled:     true,
		BearerToken: testToken,
	}, log)

	dashboardHandler := handler.NewDashboardHandler(getCurrentMetricsUC, log)
	metricsAPIHandler := handler.NewMetricsAPIHandler(getHistoricalMetricsUC, time.Hour*24, log)

	storage := newMemoryScreenshotStorage()
	saveScreenshotsUC := usecase.NewSaveDashboardScreenshotsUseCase(storage, nil, usecase.SaveDashboardScreenshotsConfig{}, log)
	listScreenshotsUC := usecase.NewListDashboardScreenshotsUseCase(storage, nil, usecase.ListDashboardScreenshotsConfig{}, log)
	screenshotAPIHandler := handler.NewScreenshotAPIHandler(
		saveScreenshotsUC,
		listScreenshotsUC,
		middleware.AuthConfig{Enabled: true, BearerToken: testToken},
		5*1024*1024,
		1*1024*1024,
		100,
		log,
	)

	authAPIHandler := handler.NewAuthAPIHandler(middleware.AuthConfig{Enabled: true, BearerToken: testToken}, log)
	releaseAnalyzerAPIHandler := handler.NewReleaseAnalyzerAPIHandler(releaseAnalyzerBaseURL, 2*time.Second, log)

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
			AuthToken:      testToken,
		},
		log,
	)

	server := httptest.NewServer(router.Setup())
	t.Cleanup(server.Close)
	return server, storage
}

func seedMetrics(t *testing.T, repo *memoryMetricRepo) {
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
			t.Fatalf("failed to build metric value: %v", err)
		}
		metric := entity.Reconstruct(entry.id, entry.metricType, entry.metricType.String(), value, nil, entry.collected, entry.collected)
		if err := repo.Save(context.Background(), metric); err != nil {
			t.Fatalf("failed to seed metrics: %v", err)
		}
	}
}

func TestE2EHealthEndpoints(t *testing.T) {
	server, _ := newTestServer(t, "http://example.invalid")

	for _, path := range []string{"/healthz", "/readyz"} {
		resp, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", path, resp.StatusCode)
		}
	}
}

func TestE2EAuthAndMetricsHistory(t *testing.T) {
	server, _ := newTestServer(t, "http://example.invalid")
	client := server.Client()

	statusResp := doRequest(t, client, http.MethodGet, server.URL+"/api/v1/auth/status", nil, nil)
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected auth status: %d", statusResp.StatusCode)
	}

	var statusPayload map[string]interface{}
	if err := json.NewDecoder(statusResp.Body).Decode(&statusPayload); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	statusResp.Body.Close()

	if statusPayload["auth_enabled"] != true {
		t.Fatalf("expected auth_enabled true, got %v", statusPayload["auth_enabled"])
	}
	if statusPayload["authenticated"] != false {
		t.Fatalf("expected authenticated false, got %v", statusPayload["authenticated"])
	}

	loginBody := bytes.NewBufferString(`{"token":"bad-token"}`)
	loginResp := doRequest(t, client, http.MethodPost, server.URL+"/api/v1/auth/login", loginBody, map[string]string{
		"Content-Type": "application/json",
	})
	if loginResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid login, got %d", loginResp.StatusCode)
	}
	loginResp.Body.Close()

	loginBody = bytes.NewBufferString(`{"token":"` + testToken + `"}`)
	loginResp = doRequest(t, client, http.MethodPost, server.URL+"/api/v1/auth/login", loginBody, map[string]string{
		"Content-Type": "application/json",
	})
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for valid login, got %d", loginResp.StatusCode)
	}
	loginResp.Body.Close()

	cookies := loginResp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected auth cookie")
	}

	authorizedStatusReq, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/auth/status", nil)
	for _, cookie := range cookies {
		authorizedStatusReq.AddCookie(cookie)
	}
	authorizedStatusResp, err := client.Do(authorizedStatusReq)
	if err != nil {
		t.Fatalf("authorized status request failed: %v", err)
	}
	defer authorizedStatusResp.Body.Close()

	var authorizedStatusPayload map[string]interface{}
	if err := json.NewDecoder(authorizedStatusResp.Body).Decode(&authorizedStatusPayload); err != nil {
		t.Fatalf("decode authorized status response: %v", err)
	}
	if authorizedStatusPayload["authenticated"] != true {
		t.Fatalf("expected authenticated true, got %v", authorizedStatusPayload["authenticated"])
	}

	historyResp := doRequest(t, client, http.MethodGet, server.URL+"/api/v1/metrics/history?type=cpu&duration=1h", nil, nil)
	if historyResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing auth, got %d", historyResp.StatusCode)
	}
	historyResp.Body.Close()

	historyResp = doRequest(t, client, http.MethodGet, server.URL+"/api/v1/metrics/history?type=cpu&duration=1h", nil, map[string]string{
		"Authorization": "Bearer " + testToken,
	})
	if historyResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for authorized metrics, got %d", historyResp.StatusCode)
	}
	defer historyResp.Body.Close()

	var history dto.MetricHistoryDTO
	if err := json.NewDecoder(historyResp.Body).Decode(&history); err != nil {
		t.Fatalf("decode history response: %v", err)
	}
	if history.Type != "cpu" {
		t.Fatalf("expected cpu history, got %s", history.Type)
	}
	if len(history.Metrics) < 2 {
		t.Fatalf("expected metrics history, got %d entries", len(history.Metrics))
	}

	legacyResp := doRequest(t, client, http.MethodGet, server.URL+"/api/metrics/history?type=cpu&duration=1h", nil, map[string]string{
		"Authorization": "Bearer " + testToken,
	})
	if legacyResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for legacy metrics endpoint, got %d", legacyResp.StatusCode)
	}
	legacyResp.Body.Close()
}

func TestE2EScreenshotEndpoints(t *testing.T) {
	server, _ := newTestServer(t, "http://example.invalid")
	client := server.Client()

	unauthorizedResp := doRequest(t, client, http.MethodPost, server.URL+"/api/v1/screenshots/dashboard", bytes.NewBufferString(`{}`), nil)
	if unauthorizedResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for screenshot without auth, got %d", unauthorizedResp.StatusCode)
	}
	unauthorizedResp.Body.Close()

	requestBody := buildScreenshotRequest(t)
	resp := doRequest(t, client, http.MethodPost, server.URL+"/api/v1/screenshots/dashboard", requestBody, map[string]string{
		"Authorization": "Bearer " + testToken,
		"Content-Type":  "application/json",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for screenshot upload, got %d", resp.StatusCode)
	}

	var saveResp struct {
		SavedAt time.Time `json:"saved_at"`
		Items   []struct {
			Type  string `json:"type"`
			S3Key string `json:"s3_key"`
			URL   string `json:"url"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&saveResp); err != nil {
		t.Fatalf("decode screenshot response: %v", err)
	}
	resp.Body.Close()

	if len(saveResp.Items) != 6 {
		t.Fatalf("expected 6 artifacts saved, got %d", len(saveResp.Items))
	}

	listResp := doRequest(t, client, http.MethodGet, server.URL+"/api/v1/screenshots/dashboard?dashboard_id=main&limit=10", nil, map[string]string{
		"Authorization": "Bearer " + testToken,
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
		t.Fatal("expected at least one screenshot item")
	}
}

func TestE2EReleaseAnalyzerProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/release-analyzer/summary":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok","summary":"ready"}`))
		case "/api/v1/release-analyzer/run":
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"queued"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(upstream.Close)

	server, _ := newTestServer(t, strings.TrimRight(upstream.URL, "/"))
	client := server.Client()

	summaryResp := doRequest(t, client, http.MethodGet, server.URL+"/api/v1/release-analyzer/summary", nil, map[string]string{
		"Authorization": "Bearer " + testToken,
	})
	if summaryResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for release analyzer summary, got %d", summaryResp.StatusCode)
	}
	summaryResp.Body.Close()

	runResp := doRequest(t, client, http.MethodPost, server.URL+"/api/v1/release-analyzer/run", nil, map[string]string{
		"Authorization": "Bearer " + testToken,
	})
	if runResp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 for release analyzer run, got %d", runResp.StatusCode)
	}
	runResp.Body.Close()
}

func buildScreenshotRequest(t *testing.T) *bytes.Buffer {
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

func doRequest(t *testing.T, client *http.Client, method, url string, body *bytes.Buffer, headers map[string]string) *http.Response {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body.Bytes())
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}
