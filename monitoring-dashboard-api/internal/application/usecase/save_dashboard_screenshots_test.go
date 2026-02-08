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

type putCall struct {
	key         string
	contentType string
	body        []byte
}

type mockScreenshotStorage struct {
	calls []putCall
	errAt map[string]error
}

func (m *mockScreenshotStorage) PutObject(_ context.Context, key, contentType string, body []byte) (string, error) {
	m.calls = append(m.calls, putCall{key: key, contentType: contentType, body: body})
	if err, ok := m.errAt[key]; ok {
		return "", err
	}
	return "https://example.com/" + key, nil
}

func (m *mockScreenshotStorage) ListObjects(_ context.Context, _ string, _ int) ([]port.ScreenshotObject, error) {
	return nil, nil
}

func (m *mockScreenshotStorage) GetObjectURL(_ context.Context, key string) (string, error) {
	return "https://example.com/" + key, nil
}

type mockScreenshotMetadataRepository struct {
	records []port.ScreenshotMetadata
	err     error
}

func (m *mockScreenshotMetadataRepository) PutBatch(_ context.Context, records []port.ScreenshotMetadata) error {
	if m.err != nil {
		return m.err
	}
	m.records = append(m.records, records...)
	return nil
}

func (m *mockScreenshotMetadataRepository) ListByDashboard(_ context.Context, _ port.ScreenshotListQuery) (port.ScreenshotListPage, error) {
	return port.ScreenshotListPage{}, nil
}

func TestSaveDashboardScreenshotsUseCase_Success(t *testing.T) {
	storage := &mockScreenshotStorage{}
	metadataRepo := &mockScreenshotMetadataRepository{}
	uc := NewSaveDashboardScreenshotsUseCase(
		storage,
		metadataRepo,
		SaveDashboardScreenshotsConfig{KeyPrefix: "dashboards"},
		logger.New("error"),
	)

	capturedAt := time.Date(2026, 2, 7, 12, 34, 56, 0, time.UTC)
	res, err := uc.Execute(context.Background(), SaveDashboardScreenshotsCommand{
		DashboardID: "main",
		CapturedAt:  capturedAt,
		Artifacts:   buildFullArtifacts(),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(res.Items) != len(requiredArtifactTypes) {
		t.Fatalf("expected %d items, got %d", len(requiredArtifactTypes), len(res.Items))
	}
	if len(storage.calls) != len(requiredArtifactTypes) {
		t.Fatalf("expected %d uploads, got %d", len(requiredArtifactTypes), len(storage.calls))
	}
	if len(metadataRepo.records) != len(requiredArtifactTypes) {
		t.Fatalf("expected %d metadata records, got %d", len(requiredArtifactTypes), len(metadataRepo.records))
	}

	expectedPrefix := "dashboards/main/2026/02/07/20260207T123456Z_"
	for _, item := range res.Items {
		if !strings.HasPrefix(item.S3Key, expectedPrefix) {
			t.Fatalf("unexpected key prefix: %s", item.S3Key)
		}
		if !strings.HasSuffix(item.S3Key, ".png") {
			t.Fatalf("unexpected key suffix: %s", item.S3Key)
		}
		if item.URL == "" {
			t.Fatalf("expected URL for %s", item.Type)
		}
	}
}

func TestSaveDashboardScreenshotsUseCase_ValidationErrors(t *testing.T) {
	storage := &mockScreenshotStorage{}
	uc := NewSaveDashboardScreenshotsUseCase(
		storage,
		nil,
		SaveDashboardScreenshotsConfig{KeyPrefix: "dashboards"},
		logger.New("error"),
	)

	tests := []struct {
		name    string
		command SaveDashboardScreenshotsCommand
		wantErr string
	}{
		{
			name: "invalid dashboard id",
			command: SaveDashboardScreenshotsCommand{
				DashboardID: "bad id with spaces",
				CapturedAt:  time.Now().UTC(),
				Artifacts:   buildFullArtifacts(),
			},
			wantErr: "invalid dashboard_id",
		},
		{
			name: "missing required artifact",
			command: SaveDashboardScreenshotsCommand{
				DashboardID: "main",
				CapturedAt:  time.Now().UTC(),
				Artifacts:   buildFullArtifacts()[:len(requiredArtifactTypes)-1],
			},
			wantErr: "missing required artifacts",
		},
		{
			name: "duplicate artifact",
			command: SaveDashboardScreenshotsCommand{
				DashboardID: "main",
				CapturedAt:  time.Now().UTC(),
				Artifacts: append(buildFullArtifacts(), ScreenshotArtifactInput{
					Type:        requiredArtifactTypes[0],
					ContentType: "image/png",
					Data:        []byte{1, 2, 3},
				}),
			},
			wantErr: "duplicate artifact type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), tc.command)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestSaveDashboardScreenshotsUseCase_StorageUploadError(t *testing.T) {
	capturedAt := time.Date(2026, 2, 7, 12, 34, 56, 0, time.UTC)
	failKey := "dashboards/main/2026/02/07/20260207T123456Z_cpu_card.png"

	storage := &mockScreenshotStorage{errAt: map[string]error{failKey: errors.New("boom")}}
	uc := NewSaveDashboardScreenshotsUseCase(
		storage,
		nil,
		SaveDashboardScreenshotsConfig{KeyPrefix: "dashboards"},
		logger.New("error"),
	)

	_, err := uc.Execute(context.Background(), SaveDashboardScreenshotsCommand{
		DashboardID: "main",
		CapturedAt:  capturedAt,
		Artifacts:   buildFullArtifacts(),
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "failed to upload cpu_card") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSaveDashboardScreenshotsUseCase_MetadataFailOpen(t *testing.T) {
	storage := &mockScreenshotStorage{}
	metadataRepo := &mockScreenshotMetadataRepository{err: errors.New("metadata unavailable")}
	uc := NewSaveDashboardScreenshotsUseCase(
		storage,
		metadataRepo,
		SaveDashboardScreenshotsConfig{
			KeyPrefix:           "dashboards",
			MetadataWriteStrict: false,
		},
		logger.New("error"),
	)

	_, err := uc.Execute(context.Background(), SaveDashboardScreenshotsCommand{
		DashboardID: "main",
		CapturedAt:  time.Now().UTC(),
		Artifacts:   buildFullArtifacts(),
	})
	if err != nil {
		t.Fatalf("expected fail-open behavior, got error: %v", err)
	}
}

func TestSaveDashboardScreenshotsUseCase_MetadataStrict(t *testing.T) {
	storage := &mockScreenshotStorage{}
	metadataRepo := &mockScreenshotMetadataRepository{err: errors.New("metadata unavailable")}
	uc := NewSaveDashboardScreenshotsUseCase(
		storage,
		metadataRepo,
		SaveDashboardScreenshotsConfig{
			KeyPrefix:           "dashboards",
			MetadataWriteStrict: true,
		},
		logger.New("error"),
	)

	_, err := uc.Execute(context.Background(), SaveDashboardScreenshotsCommand{
		DashboardID: "main",
		CapturedAt:  time.Now().UTC(),
		Artifacts:   buildFullArtifacts(),
	})
	if err == nil || !strings.Contains(err.Error(), "failed to save screenshot metadata") {
		t.Fatalf("expected strict metadata error, got %v", err)
	}
}

func buildFullArtifacts() []ScreenshotArtifactInput {
	artifacts := make([]ScreenshotArtifactInput, 0, len(requiredArtifactTypes))
	for _, artifactType := range requiredArtifactTypes {
		artifacts = append(artifacts, ScreenshotArtifactInput{
			Type:        artifactType,
			ContentType: "image/png",
			Data:        []byte{1, 2, 3, 4},
		})
	}
	return artifacts
}
