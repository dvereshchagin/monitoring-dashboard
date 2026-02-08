package handler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dreschagin/monitoring-dashboard/internal/interfaces/http/middleware"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
)

const maxReleaseAnalyzerResponseBytes = 2 * 1024 * 1024

type ReleaseAnalyzerAPIHandler struct {
	baseURL string
	client  *http.Client
	logger  *logger.Logger
}

func NewReleaseAnalyzerAPIHandler(baseURL string, timeout time.Duration, log *logger.Logger) *ReleaseAnalyzerAPIHandler {
	if timeout <= 0 {
		timeout = 6 * time.Second
	}

	return &ReleaseAnalyzerAPIHandler{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client: &http.Client{
			Timeout: timeout,
		},
		logger: log,
	}
}

func (h *ReleaseAnalyzerAPIHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.proxy(r.Context(), w, http.MethodGet, "/api/v1/release-analyzer/summary")
}

func (h *ReleaseAnalyzerAPIHandler) RunNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.proxy(r.Context(), w, http.MethodPost, "/api/v1/release-analyzer/run")
}

func (h *ReleaseAnalyzerAPIHandler) proxy(ctx context.Context, w http.ResponseWriter, method string, path string) {
	if h.baseURL == "" {
		middleware.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "release analyzer base URL is not configured",
		})
		return
	}

	req, err := http.NewRequestWithContext(ctx, method, h.baseURL+path, nil)
	if err != nil {
		h.logger.Error("Failed to build release analyzer request", err, "path", path)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to build release analyzer request",
		})
		return
	}
	req.Header.Set("Accept", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		h.logger.Error("Release analyzer request failed", err, "path", path)
		middleware.WriteJSON(w, http.StatusBadGateway, map[string]string{
			"error": "release analyzer is unavailable",
		})
		return
	}
	defer resp.Body.Close()

	body, err := readLimited(resp.Body, maxReleaseAnalyzerResponseBytes)
	if err != nil {
		h.logger.Error("Failed to read release analyzer response body", err, "path", path)
		middleware.WriteJSON(w, http.StatusBadGateway, map[string]string{
			"error": "failed to read release analyzer response",
		})
		return
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/json"
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(resp.StatusCode)
	if _, err := w.Write(body); err != nil {
		h.logger.Error("Failed to write release analyzer response to client", err, "path", path)
	}
}

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	lr := io.LimitReader(r, limit+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, io.ErrUnexpectedEOF
	}
	return data, nil
}
