package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dreschagin/monitoring-dashboard/internal/application/usecase"
	"github.com/dreschagin/monitoring-dashboard/internal/interfaces/http/middleware"
	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
)

type ScreenshotAPIHandler struct {
	saveDashboardScreenshotsUC *usecase.SaveDashboardScreenshotsUseCase
	authConfig                 middleware.AuthConfig
	logger                     *logger.Logger
	maxPayloadBytes            int64
	maxArtifactBytes           int
	rateLimiter                *fixedWindowRateLimiter
}

type screenshotRequest struct {
	DashboardID string                      `json:"dashboard_id"`
	CapturedAt  time.Time                   `json:"captured_at"`
	Artifacts   []screenshotArtifactRequest `json:"artifacts"`
}

type screenshotArtifactRequest struct {
	Type        string `json:"type"`
	ContentType string `json:"content_type"`
	DataBase64  string `json:"data_base64"`
}

type screenshotResponse struct {
	SavedAt time.Time                `json:"saved_at"`
	Items   []screenshotResponseItem `json:"items"`
}

type screenshotResponseItem struct {
	Type  string `json:"type"`
	S3Key string `json:"s3_key"`
	URL   string `json:"url"`
}

func NewScreenshotAPIHandler(
	saveDashboardScreenshotsUC *usecase.SaveDashboardScreenshotsUseCase,
	authConfig middleware.AuthConfig,
	maxPayloadBytes int64,
	maxArtifactBytes int,
	rateLimitPerMinute int,
	log *logger.Logger,
) *ScreenshotAPIHandler {
	if maxPayloadBytes <= 0 {
		maxPayloadBytes = 20 * 1024 * 1024
	}
	if maxArtifactBytes <= 0 {
		maxArtifactBytes = 5 * 1024 * 1024
	}
	if rateLimitPerMinute <= 0 {
		rateLimitPerMinute = 30
	}

	return &ScreenshotAPIHandler{
		saveDashboardScreenshotsUC: saveDashboardScreenshotsUC,
		authConfig:                 authConfig,
		logger:                     log,
		maxPayloadBytes:            maxPayloadBytes,
		maxArtifactBytes:           maxArtifactBytes,
		rateLimiter:                newFixedWindowRateLimiter(rateLimitPerMinute, time.Minute),
	}
}

func (h *ScreenshotAPIHandler) SaveDashboardScreenshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := middleware.ValidateRequestAuth(r, h.authConfig); err != nil {
		w.Header().Set("WWW-Authenticate", `Bearer realm="monitoring-dashboard"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	clientIP := extractClientIP(r)
	if !h.rateLimiter.Allow(clientIP) {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxPayloadBytes)
	defer r.Body.Close()

	var req screenshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			http.Error(w, "Payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	artifacts := make([]usecase.ScreenshotArtifactInput, 0, len(req.Artifacts))
	for _, artifact := range req.Artifacts {
		decoded, err := decodeBase64Image(artifact.DataBase64)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid artifact %s: %v", artifact.Type, err), http.StatusBadRequest)
			return
		}

		if len(decoded) > h.maxArtifactBytes {
			http.Error(w, "Artifact too large", http.StatusRequestEntityTooLarge)
			return
		}

		artifacts = append(artifacts, usecase.ScreenshotArtifactInput{
			Type:        artifact.Type,
			ContentType: artifact.ContentType,
			Data:        decoded,
		})
	}

	result, err := h.saveDashboardScreenshotsUC.Execute(r.Context(), usecase.SaveDashboardScreenshotsCommand{
		DashboardID: req.DashboardID,
		CapturedAt:  req.CapturedAt,
		Artifacts:   artifacts,
	})
	if err != nil {
		h.logger.Error("Failed to save dashboard screenshots", err,
			"dashboard_id", req.DashboardID,
			"artifacts_count", len(req.Artifacts),
		)
		statusCode := http.StatusBadRequest
		if strings.Contains(err.Error(), "failed to upload") {
			statusCode = http.StatusInternalServerError
		}
		if strings.Contains(err.Error(), "not configured") {
			statusCode = http.StatusServiceUnavailable
		}
		http.Error(w, err.Error(), statusCode)
		return
	}

	items := make([]screenshotResponseItem, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, screenshotResponseItem{
			Type:  item.Type,
			S3Key: item.S3Key,
			URL:   item.URL,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(screenshotResponse{
		SavedAt: result.SavedAt,
		Items:   items,
	}); err != nil {
		h.logger.Error("Failed to encode screenshot response", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func decodeBase64Image(raw string) ([]byte, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, fmt.Errorf("empty data_base64")
	}

	value = strings.TrimPrefix(value, "data:image/png;base64,")

	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("invalid base64")
	}

	if len(decoded) < 8 || !bytes.Equal(decoded[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return nil, fmt.Errorf("invalid png signature")
	}

	return decoded, nil
}

func extractClientIP(r *http.Request) string {
	xForwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xForwardedFor != "" {
		parts := strings.Split(xForwardedFor, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

type fixedWindowRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]*rateLimitEntry
}

type rateLimitEntry struct {
	windowStart time.Time
	count       int
}

func newFixedWindowRateLimiter(limit int, window time.Duration) *fixedWindowRateLimiter {
	return &fixedWindowRateLimiter{
		limit:   limit,
		window:  window,
		entries: make(map[string]*rateLimitEntry),
	}
}

func (rl *fixedWindowRateLimiter) Allow(key string) bool {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, ok := rl.entries[key]
	if !ok || now.Sub(entry.windowStart) >= rl.window {
		rl.entries[key] = &rateLimitEntry{windowStart: now, count: 1}
		return true
	}

	if entry.count >= rl.limit {
		return false
	}

	entry.count++
	return true
}
