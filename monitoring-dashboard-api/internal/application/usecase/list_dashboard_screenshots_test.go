package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dreschagin/monitoring-dashboard/internal/application/port"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
)

type listMockScreenshotStorage struct {
	objectsByPrefix map[string][]port.ScreenshotObject
	err             error
	lastPrefix      string
	lastLimit       int
}

func (m *listMockScreenshotStorage) PutObject(_ context.Context, _, _ string, _ []byte) (string, error) {
	return "", nil
}

func (m *listMockScreenshotStorage) ListObjects(_ context.Context, prefix string, limit int) ([]port.ScreenshotObject, error) {
	m.lastPrefix = prefix
	m.lastLimit = limit

	if m.err != nil {
		return nil, m.err
	}
	return m.objectsByPrefix[prefix], nil
}

func (m *listMockScreenshotStorage) GetObjectURL(_ context.Context, key string) (string, error) {
	return "https://signed.example.com/" + key, nil
}

type listMockScreenshotMetadataRepository struct {
	page      port.ScreenshotListPage
	err       error
	lastQuery port.ScreenshotListQuery
}

func (m *listMockScreenshotMetadataRepository) PutBatch(_ context.Context, _ []port.ScreenshotMetadata) error {
	return nil
}

func (m *listMockScreenshotMetadataRepository) ListByDashboard(_ context.Context, query port.ScreenshotListQuery) (port.ScreenshotListPage, error) {
	m.lastQuery = query
	if m.err != nil {
		return port.ScreenshotListPage{}, m.err
	}
	return m.page, nil
}

func TestListDashboardScreenshotsUseCase_Success(t *testing.T) {
	storage := &listMockScreenshotStorage{
		objectsByPrefix: map[string][]port.ScreenshotObject{
			"dashboards/main/": {
				{
					Key:          "dashboards/main/2026/02/08/20260208T090500Z_cpu_card.png",
					URL:          "https://example.com/2",
					LastModified: time.Date(2026, 2, 8, 9, 10, 0, 0, time.UTC),
				},
				{
					Key:          "dashboards/main/2026/02/08/20260208T090400Z_memory_card.png",
					URL:          "https://example.com/1",
					LastModified: time.Date(2026, 2, 8, 9, 9, 0, 0, time.UTC),
				},
			},
		},
	}

	uc := NewListDashboardScreenshotsUseCase(
		storage,
		nil,
		ListDashboardScreenshotsConfig{
			KeyPrefix:           "dashboards",
			DefaultLimit:        24,
			MaxLimit:            100,
			FallbackToS3OnError: true,
		},
		logger.New("error"),
	)

	res, err := uc.Execute(context.Background(), ListDashboardScreenshotsCommand{
		DashboardID: "main",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if storage.lastPrefix != "dashboards/main/" {
		t.Fatalf("unexpected prefix: %s", storage.lastPrefix)
	}
	if storage.lastLimit != 24 {
		t.Fatalf("unexpected limit: %d", storage.lastLimit)
	}
	if len(res.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(res.Items))
	}
	if res.Items[0].Type != "cpu_card" {
		t.Fatalf("expected first type cpu_card, got %s", res.Items[0].Type)
	}
	if res.Items[0].CapturedAt.IsZero() {
		t.Fatalf("expected captured_at to be parsed")
	}
	if !res.Items[0].LastModified.After(res.Items[1].LastModified) {
		t.Fatalf("expected result sorted by last_modified desc")
	}
}

func TestListDashboardScreenshotsUseCase_ValidationAndLimit(t *testing.T) {
	storage := &listMockScreenshotStorage{}
	uc := NewListDashboardScreenshotsUseCase(
		storage,
		nil,
		ListDashboardScreenshotsConfig{
			KeyPrefix:           "dashboards",
			DefaultLimit:        10,
			MaxLimit:            50,
			FallbackToS3OnError: true,
		},
		logger.New("error"),
	)

	_, err := uc.Execute(context.Background(), ListDashboardScreenshotsCommand{
		DashboardID: "invalid id",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid dashboard_id") {
		t.Fatalf("expected invalid dashboard_id error, got %v", err)
	}

	_, err = uc.Execute(context.Background(), ListDashboardScreenshotsCommand{
		DashboardID: "main",
		Limit:       500,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if storage.lastLimit != 50 {
		t.Fatalf("expected clamped limit 50, got %d", storage.lastLimit)
	}
}

func TestListDashboardScreenshotsUseCase_StorageError(t *testing.T) {
	storage := &listMockScreenshotStorage{err: errors.New("boom")}
	uc := NewListDashboardScreenshotsUseCase(
		storage,
		nil,
		ListDashboardScreenshotsConfig{
			KeyPrefix:           "dashboards",
			DefaultLimit:        24,
			MaxLimit:            100,
			FallbackToS3OnError: true,
		},
		logger.New("error"),
	)

	_, err := uc.Execute(context.Background(), ListDashboardScreenshotsCommand{
		DashboardID: "main",
	})
	if err == nil || !strings.Contains(err.Error(), "failed to list screenshots") {
		t.Fatalf("expected storage error wrapper, got %v", err)
	}
}

func TestListDashboardScreenshotsUseCase_MetadataPrimary(t *testing.T) {
	metadataRepo := &listMockScreenshotMetadataRepository{
		page: port.ScreenshotListPage{
			Items: []port.ScreenshotMetadata{
				{
					DashboardID:  "main",
					ArtifactType: "cpu_card",
					S3Key:        "dashboards/main/2026/02/08/20260208T090500Z_cpu_card.png",
					CapturedAt:   time.Date(2026, 2, 8, 9, 5, 0, 0, time.UTC),
					LastModified: time.Date(2026, 2, 8, 9, 5, 1, 0, time.UTC),
				},
			},
			NextCursor: "next-page",
		},
	}
	storage := &listMockScreenshotStorage{}
	uc := NewListDashboardScreenshotsUseCase(
		storage,
		metadataRepo,
		ListDashboardScreenshotsConfig{
			KeyPrefix:           "dashboards",
			DefaultLimit:        24,
			MaxLimit:            100,
			FallbackToS3OnError: true,
		},
		logger.New("error"),
	)

	res, err := uc.Execute(context.Background(), ListDashboardScreenshotsCommand{
		DashboardID:  "main",
		Limit:        10,
		ArtifactType: "cpu_card",
		Cursor:       "cursor",
		From:         time.Date(2026, 2, 8, 0, 0, 0, 0, time.UTC),
		To:           time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if res.NextCursor != "next-page" {
		t.Fatalf("unexpected next cursor: %s", res.NextCursor)
	}
	if len(res.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Items))
	}
	if !strings.HasPrefix(res.Items[0].URL, "https://signed.example.com/") {
		t.Fatalf("expected signed URL, got %s", res.Items[0].URL)
	}
	if metadataRepo.lastQuery.ArtifactType != "cpu_card" {
		t.Fatalf("expected artifact_type filter to be passed")
	}
}

func TestListDashboardScreenshotsUseCase_MetadataFallbackToS3(t *testing.T) {
	metadataRepo := &listMockScreenshotMetadataRepository{err: errors.New("ddb down")}
	storage := &listMockScreenshotStorage{
		objectsByPrefix: map[string][]port.ScreenshotObject{
			"dashboards/main/": {
				{
					Key:          "dashboards/main/2026/02/08/20260208T090500Z_cpu_card.png",
					URL:          "https://example.com/2",
					LastModified: time.Date(2026, 2, 8, 9, 10, 0, 0, time.UTC),
				},
			},
		},
	}
	uc := NewListDashboardScreenshotsUseCase(
		storage,
		metadataRepo,
		ListDashboardScreenshotsConfig{
			KeyPrefix:           "dashboards",
			DefaultLimit:        24,
			MaxLimit:            100,
			FallbackToS3OnError: true,
		},
		logger.New("error"),
	)

	res, err := uc.Execute(context.Background(), ListDashboardScreenshotsCommand{
		DashboardID: "main",
		Limit:       24,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Items))
	}
}

func TestListDashboardScreenshotsUseCase_MetadataNoFallback(t *testing.T) {
	metadataRepo := &listMockScreenshotMetadataRepository{err: errors.New("ddb down")}
	storage := &listMockScreenshotStorage{}
	uc := NewListDashboardScreenshotsUseCase(
		storage,
		metadataRepo,
		ListDashboardScreenshotsConfig{
			KeyPrefix:           "dashboards",
			DefaultLimit:        24,
			MaxLimit:            100,
			FallbackToS3OnError: false,
		},
		logger.New("error"),
	)

	_, err := uc.Execute(context.Background(), ListDashboardScreenshotsCommand{
		DashboardID: "main",
		Limit:       24,
	})
	if err == nil || !strings.Contains(err.Error(), "metadata index") {
		t.Fatalf("expected metadata index error, got %v", err)
	}
}

func TestInferHelpers(t *testing.T) {
	key := "dashboards/main/2026/02/08/20260208T090500Z_cpu_chart.png"
	if got := inferArtifactType(key); got != "cpu_chart" {
		t.Fatalf("unexpected type: %s", got)
	}

	captured := inferCapturedAt(key)
	want := time.Date(2026, 2, 8, 9, 5, 0, 0, time.UTC)
	if !captured.Equal(want) {
		t.Fatalf("unexpected captured_at: %s", captured)
	}
}
