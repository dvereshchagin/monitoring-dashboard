package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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

func TestSaveDashboardScreenshotsUseCase_Success(t *testing.T) {
	storage := &mockScreenshotStorage{}
	uc := NewSaveDashboardScreenshotsUseCase(storage, SaveDashboardScreenshotsConfig{KeyPrefix: "dashboards"}, logger.New("error"))

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
	uc := NewSaveDashboardScreenshotsUseCase(storage, SaveDashboardScreenshotsConfig{KeyPrefix: "dashboards"}, logger.New("error"))

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
	uc := NewSaveDashboardScreenshotsUseCase(storage, SaveDashboardScreenshotsConfig{KeyPrefix: "dashboards"}, logger.New("error"))

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
