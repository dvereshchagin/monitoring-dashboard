package port

import (
	"context"
	"time"
)

// ScreenshotMetadata представляет метаданные артефакта скриншота.
type ScreenshotMetadata struct {
	DashboardID  string
	ArtifactType string
	S3Key        string
	URL          string
	ContentType  string
	SizeBytes    int64
	CapturedAt   time.Time
	LastModified time.Time
	ExpiresAt    time.Time
}

// ScreenshotListQuery определяет параметры выборки списка скриншотов.
type ScreenshotListQuery struct {
	DashboardID  string
	Limit        int
	Cursor       string
	ArtifactType string
	From         time.Time
	To           time.Time
}

// ScreenshotListPage содержит результат выборки и курсор следующей страницы.
type ScreenshotListPage struct {
	Items      []ScreenshotMetadata
	NextCursor string
}

// ScreenshotMetadataRepository определяет интерфейс хранения метаданных скриншотов.
type ScreenshotMetadataRepository interface {
	PutBatch(ctx context.Context, records []ScreenshotMetadata) error
	ListByDashboard(ctx context.Context, query ScreenshotListQuery) (ScreenshotListPage, error)
}
