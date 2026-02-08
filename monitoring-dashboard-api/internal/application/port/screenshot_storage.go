package port

import (
	"context"
	"time"
)

type ScreenshotObject struct {
	Key          string
	LastModified time.Time
	URL          string
}

// ScreenshotStorage определяет интерфейс для хранения скриншотов.
type ScreenshotStorage interface {
	// PutObject загружает объект и возвращает URL для чтения.
	PutObject(ctx context.Context, key, contentType string, body []byte) (string, error)
	// ListObjects возвращает список объектов по префиксу.
	ListObjects(ctx context.Context, prefix string, limit int) ([]ScreenshotObject, error)
	// GetObjectURL возвращает URL для чтения объекта по ключу.
	GetObjectURL(ctx context.Context, key string) (string, error)
}
