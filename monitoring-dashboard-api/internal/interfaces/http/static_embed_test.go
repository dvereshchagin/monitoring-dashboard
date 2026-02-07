package http

import (
	"io/fs"
	"testing"
)

func TestEmbeddedStaticFiles(t *testing.T) {
	if _, err := fs.ReadFile(staticFiles, "static/css/style.css"); err != nil {
		t.Fatalf("expected embedded css asset, got error: %v", err)
	}

	if _, err := fs.ReadFile(staticFiles, "static/js/websocket.js"); err != nil {
		t.Fatalf("expected embedded js asset, got error: %v", err)
	}
}
