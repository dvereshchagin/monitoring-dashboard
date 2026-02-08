package releaseanalyzer

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Handler struct {
	runner *Runner
}

func NewHandler(runner *Runner) *Handler {
	return &Handler{runner: runner}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", h.healthz)
	mux.HandleFunc("/readyz", h.readyz)
	mux.HandleFunc("/api/v1/release-analyzer/summary", h.summary)
	mux.HandleFunc("/api/v1/release-analyzer/run", h.runNow)

	return mux
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshot := h.runner.Snapshot()

	response := map[string]string{
		"status":     "ok",
		"uptime":     time.Since(snapshot.StartedAt).Round(time.Second).String(),
		"last_run":   snapshot.LastRunAt.UTC().Format(time.RFC3339),
		"last_error": snapshot.LastError,
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) readyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshot := h.runner.Snapshot()
	if snapshot.LastRunAt.IsZero() {
		http.Error(w, "not ready: no successful cycle yet", http.StatusServiceUnavailable)
		return
	}
	if time.Since(snapshot.LastRunAt) > snapshot.Interval*3 {
		http.Error(w, "not ready: stale analyzer cycle", http.StatusServiceUnavailable)
		return
	}
	if snapshot.LastError != "" {
		http.Error(w, "not ready: last cycle failed", http.StatusServiceUnavailable)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, h.runner.Snapshot())
}

func (h *Handler) runNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	summary, err := h.runner.RunOnce(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(data)
}
