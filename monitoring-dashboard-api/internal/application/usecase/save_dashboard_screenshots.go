package usecase

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dreschagin/monitoring-dashboard/internal/application/port"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
)

var (
	dashboardIDRegex      = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
	requiredArtifactTypes = []string{
		"cpu_card",
		"memory_card",
		"disk_card",
		"network_card",
		"cpu_chart",
		"memory_chart",
	}
)

type ScreenshotArtifactInput struct {
	Type        string
	ContentType string
	Data        []byte
}

type SaveDashboardScreenshotsCommand struct {
	DashboardID string
	CapturedAt  time.Time
	Artifacts   []ScreenshotArtifactInput
}

type SavedScreenshotItem struct {
	Type  string
	S3Key string
	URL   string
}

type SaveDashboardScreenshotsResult struct {
	SavedAt time.Time
	Items   []SavedScreenshotItem
}

type SaveDashboardScreenshotsConfig struct {
	KeyPrefix string
}

type SaveDashboardScreenshotsUseCase struct {
	storage port.ScreenshotStorage
	config  SaveDashboardScreenshotsConfig
	logger  *logger.Logger
}

func NewSaveDashboardScreenshotsUseCase(
	storage port.ScreenshotStorage,
	config SaveDashboardScreenshotsConfig,
	log *logger.Logger,
) *SaveDashboardScreenshotsUseCase {
	return &SaveDashboardScreenshotsUseCase{
		storage: storage,
		config:  config,
		logger:  log,
	}
}

func (uc *SaveDashboardScreenshotsUseCase) Execute(
	ctx context.Context,
	cmd SaveDashboardScreenshotsCommand,
) (*SaveDashboardScreenshotsResult, error) {
	if uc.storage == nil {
		return nil, fmt.Errorf("screenshot storage is not configured")
	}

	dashboardID := strings.TrimSpace(cmd.DashboardID)
	if !dashboardIDRegex.MatchString(dashboardID) {
		return nil, fmt.Errorf("invalid dashboard_id")
	}

	capturedAt := cmd.CapturedAt.UTC()
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}

	artifactsByType := make(map[string]ScreenshotArtifactInput, len(cmd.Artifacts))
	for _, artifact := range cmd.Artifacts {
		artifactType := strings.TrimSpace(artifact.Type)
		if artifactType == "" {
			return nil, fmt.Errorf("artifact type is required")
		}

		if !isRequiredArtifactType(artifactType) {
			return nil, fmt.Errorf("unsupported artifact type: %s", artifactType)
		}

		if artifact.ContentType != "image/png" {
			return nil, fmt.Errorf("unsupported content_type for %s", artifactType)
		}

		if len(artifact.Data) == 0 {
			return nil, fmt.Errorf("artifact %s is empty", artifactType)
		}

		if _, exists := artifactsByType[artifactType]; exists {
			return nil, fmt.Errorf("duplicate artifact type: %s", artifactType)
		}

		artifactsByType[artifactType] = artifact
	}

	if err := ensureRequiredArtifacts(artifactsByType); err != nil {
		return nil, err
	}

	items := make([]SavedScreenshotItem, 0, len(requiredArtifactTypes))
	for _, artifactType := range requiredArtifactTypes {
		artifact := artifactsByType[artifactType]
		key := uc.buildS3Key(dashboardID, capturedAt, artifactType)

		url, err := uc.storage.PutObject(ctx, key, artifact.ContentType, artifact.Data)
		if err != nil {
			uc.logger.Error("Failed to upload dashboard screenshot", err,
				"dashboard_id", dashboardID,
				"artifact_type", artifactType,
			)
			return nil, fmt.Errorf("failed to upload %s: %w", artifactType, err)
		}

		items = append(items, SavedScreenshotItem{
			Type:  artifactType,
			S3Key: key,
			URL:   url,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Type < items[j].Type
	})

	return &SaveDashboardScreenshotsResult{
		SavedAt: time.Now().UTC(),
		Items:   items,
	}, nil
}

func (uc *SaveDashboardScreenshotsUseCase) buildS3Key(dashboardID string, capturedAt time.Time, artifactType string) string {
	prefix := strings.Trim(uc.config.KeyPrefix, "/")
	if prefix == "" {
		prefix = "dashboards"
	}

	timestamp := capturedAt.Format("20060102T150405Z")
	datePrefix := capturedAt.Format("2006/01/02")

	return fmt.Sprintf("%s/%s/%s/%s_%s.png", prefix, dashboardID, datePrefix, timestamp, artifactType)
}

func isRequiredArtifactType(artifactType string) bool {
	for _, required := range requiredArtifactTypes {
		if artifactType == required {
			return true
		}
	}
	return false
}

func ensureRequiredArtifacts(artifactsByType map[string]ScreenshotArtifactInput) error {
	missing := make([]string, 0)
	for _, required := range requiredArtifactTypes {
		if _, ok := artifactsByType[required]; !ok {
			missing = append(missing, required)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required artifacts: %s", strings.Join(missing, ","))
	}

	return nil
}
