package releaseanalyzer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dreschagin/monitoring-dashboard/pkg/logger"
)

type Runner struct {
	service  *Service
	log      *logger.Logger
	interval time.Duration

	runMu sync.Mutex

	mu          sync.RWMutex
	startedAt   time.Time
	lastRunAt   time.Time
	lastError   string
	lastSummary *CycleSummary
}

func NewRunner(service *Service, log *logger.Logger, interval time.Duration) *Runner {
	return &Runner{
		service:   service,
		log:       log,
		interval:  interval,
		startedAt: time.Now(),
	}
}

func (r *Runner) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if _, err := r.RunOnce(ctx); err != nil {
				// RunOnce already stores error state and logs context.
				continue
			}
		case <-ctx.Done():
			return
		}
	}
}

func (r *Runner) RunOnce(ctx context.Context) (*CycleSummary, error) {
	r.runMu.Lock()
	defer r.runMu.Unlock()

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	summary, err := r.service.EvaluateLatest(queryCtx)
	runAt := time.Now()

	if err != nil {
		wrappedErr := fmt.Errorf("analyzer cycle failed: %w", err)
		r.updateFailure(runAt, wrappedErr)
		r.log.Error("Release analyzer cycle failed", wrappedErr)
		return nil, wrappedErr
	}

	r.updateSuccess(runAt, summary)

	if summary.MetricsTotal == 0 {
		r.log.Warn("Release analyzer cycle completed with empty metrics set")
		return summary, nil
	}

	r.log.Info(
		"Release analyzer cycle completed",
		"metrics_total", summary.MetricsTotal,
		"critical_count", summary.CriticalCount,
		"warning_count", summary.WarningCount,
		"oldest_metric_age", summary.OldestMetricAge.String(),
	)

	return summary, nil
}

func (r *Runner) Snapshot() Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshot := Snapshot{
		StartedAt: r.startedAt,
		Interval:  r.interval,
		LastRunAt: r.lastRunAt,
		LastError: r.lastError,
	}

	if r.lastSummary != nil {
		copiedSummary := *r.lastSummary
		copiedSummary.Assessments = append([]MetricAssessment(nil), r.lastSummary.Assessments...)
		snapshot.LastSummary = &copiedSummary
	}

	return snapshot
}

func (r *Runner) updateFailure(runAt time.Time, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lastRunAt = runAt
	r.lastError = err.Error()
}

func (r *Runner) updateSuccess(runAt time.Time, summary *CycleSummary) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lastRunAt = runAt
	r.lastError = ""
	r.lastSummary = summary
}
