package port

import "context"

// ScreenshotStorage определяет интерфейс для хранения скриншотов.
type ScreenshotStorage interface {
	// PutObject загружает объект и возвращает URL для чтения.
	PutObject(ctx context.Context, key, contentType string, body []byte) (string, error)
}
