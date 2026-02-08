package usecase

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/dreschagin/monitoring-dashboard/internal/application/port"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
)

type ListDashboardScreenshotsCommand struct {
	DashboardID  string
	Limit        int
	Cursor       string
	ArtifactType string
	From         time.Time
	To           time.Time
}

type DashboardScreenshotListItem struct {
	Type         string
	S3Key        string
	URL          string
	CapturedAt   time.Time
	LastModified time.Time
}

type ListDashboardScreenshotsResult struct {
	Items      []DashboardScreenshotListItem
	NextCursor string
}

type ListDashboardScreenshotsConfig struct {
	KeyPrefix           string
	DefaultLimit        int
	MaxLimit            int
	FallbackToS3OnError bool
}

type ListDashboardScreenshotsUseCase struct {
	storage            port.ScreenshotStorage
	metadataRepository port.ScreenshotMetadataRepository
	config             ListDashboardScreenshotsConfig
	logger             *logger.Logger
}

func NewListDashboardScreenshotsUseCase(
	storage port.ScreenshotStorage,
	metadataRepository port.ScreenshotMetadataRepository,
	config ListDashboardScreenshotsConfig,
	log *logger.Logger,
) *ListDashboardScreenshotsUseCase {
	if config.DefaultLimit <= 0 {
		config.DefaultLimit = 24
	}
	if config.MaxLimit <= 0 {
		config.MaxLimit = 100
	}
	return &ListDashboardScreenshotsUseCase{
		storage:            storage,
		metadataRepository: metadataRepository,
		config:             config,
		logger:             log,
	}
}

func (uc *ListDashboardScreenshotsUseCase) Execute(
	ctx context.Context,
	cmd ListDashboardScreenshotsCommand,
) (*ListDashboardScreenshotsResult, error) {
	dashboardID := strings.TrimSpace(cmd.DashboardID)
	if !dashboardIDRegex.MatchString(dashboardID) {
		return nil, fmt.Errorf("invalid dashboard_id")
	}

	limit := cmd.Limit
	if limit <= 0 {
		limit = uc.config.DefaultLimit
	}
	if limit > uc.config.MaxLimit {
		limit = uc.config.MaxLimit
	}

	if !cmd.From.IsZero() && !cmd.To.IsZero() && cmd.From.After(cmd.To) {
		return nil, fmt.Errorf("from must be less than or equal to to")
	}

	metadataQuery := port.ScreenshotListQuery{
		DashboardID:  dashboardID,
		Limit:        limit,
		Cursor:       strings.TrimSpace(cmd.Cursor),
		ArtifactType: strings.TrimSpace(cmd.ArtifactType),
		From:         cmd.From.UTC(),
		To:           cmd.To.UTC(),
	}

	if uc.metadataRepository != nil {
		page, err := uc.metadataRepository.ListByDashboard(ctx, metadataQuery)
		if err == nil {
			return uc.mapMetadataPage(ctx, page), nil
		}

		if !uc.config.FallbackToS3OnError {
			return nil, fmt.Errorf("failed to list screenshots via metadata index: %w", err)
		}

		if uc.logger != nil {
			uc.logger.Warn("Screenshot metadata index is unavailable, using S3 fallback",
				"dashboard_id", dashboardID,
				"error", err.Error(),
			)
		}
	}

	return uc.listFromS3(ctx, metadataQuery)
}

func (uc *ListDashboardScreenshotsUseCase) buildPrefix(dashboardID string) string {
	prefix := strings.Trim(uc.config.KeyPrefix, "/")
	if prefix == "" {
		prefix = "dashboards"
	}
	return fmt.Sprintf("%s/%s/", prefix, dashboardID)
}

func (uc *ListDashboardScreenshotsUseCase) mapMetadataPage(
	ctx context.Context,
	page port.ScreenshotListPage,
) *ListDashboardScreenshotsResult {
	items := make([]DashboardScreenshotListItem, 0, len(page.Items))
	for _, record := range page.Items {
		url := record.URL
		if uc.storage != nil {
			if generatedURL, err := uc.storage.GetObjectURL(ctx, record.S3Key); err == nil {
				url = generatedURL
			}
		}

		items = append(items, DashboardScreenshotListItem{
			Type:         record.ArtifactType,
			S3Key:        record.S3Key,
			URL:          url,
			CapturedAt:   record.CapturedAt.UTC(),
			LastModified: record.LastModified.UTC(),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].CapturedAt.After(items[j].CapturedAt)
	})

	return &ListDashboardScreenshotsResult{
		Items:      items,
		NextCursor: page.NextCursor,
	}
}

func (uc *ListDashboardScreenshotsUseCase) listFromS3(
	ctx context.Context,
	query port.ScreenshotListQuery,
) (*ListDashboardScreenshotsResult, error) {
	if uc.storage == nil {
		return nil, fmt.Errorf("screenshot storage is not configured")
	}
	if strings.TrimSpace(query.Cursor) != "" {
		return nil, fmt.Errorf("cursor pagination requires screenshot metadata index")
	}

	prefix := uc.buildPrefix(query.DashboardID)
	objects, err := uc.storage.ListObjects(ctx, prefix, query.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list screenshots: %w", err)
	}

	filtered := make([]DashboardScreenshotListItem, 0, len(objects))
	for _, object := range objects {
		item := DashboardScreenshotListItem{
			Type:         inferArtifactType(object.Key),
			S3Key:        object.Key,
			URL:          object.URL,
			CapturedAt:   inferCapturedAt(object.Key),
			LastModified: object.LastModified.UTC(),
		}

		if query.ArtifactType != "" && item.Type != query.ArtifactType {
			continue
		}
		if !query.From.IsZero() && item.CapturedAt.Before(query.From) {
			continue
		}
		if !query.To.IsZero() && item.CapturedAt.After(query.To) {
			continue
		}

		filtered = append(filtered, item)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].LastModified.After(filtered[j].LastModified)
	})

	if len(filtered) > query.Limit {
		filtered = filtered[:query.Limit]
	}

	return &ListDashboardScreenshotsResult{
		Items:      filtered,
		NextCursor: "",
	}, nil
}

func inferArtifactType(key string) string {
	filename := path.Base(strings.TrimSpace(key))
	if filename == "" || filename == "." {
		return "unknown"
	}
	if !strings.HasSuffix(filename, ".png") {
		return "unknown"
	}

	withoutExt := strings.TrimSuffix(filename, ".png")
	underscore := strings.IndexRune(withoutExt, '_')
	if underscore <= 0 || underscore == len(withoutExt)-1 {
		return "unknown"
	}
	return withoutExt[underscore+1:]
}

func inferCapturedAt(key string) time.Time {
	filename := path.Base(strings.TrimSpace(key))
	if filename == "" || filename == "." {
		return time.Time{}
	}

	withoutExt := strings.TrimSuffix(filename, ".png")
	underscore := strings.IndexRune(withoutExt, '_')
	if underscore <= 0 {
		return time.Time{}
	}

	ts := withoutExt[:underscore]
	capturedAt, err := time.Parse("20060102T150405Z", ts)
	if err != nil {
		return time.Time{}
	}
	return capturedAt.UTC()
}
