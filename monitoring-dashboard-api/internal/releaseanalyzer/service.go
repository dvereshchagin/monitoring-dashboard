package releaseanalyzer

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const latestMetricsQuery = `
	SELECT DISTINCT ON (metric_type)
		metric_type, value, unit, collected_at
	FROM metrics
	ORDER BY metric_type, collected_at DESC
`

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) EvaluateLatest(ctx context.Context) (*CycleSummary, error) {
	rows, err := s.db.QueryContext(ctx, latestMetricsQuery)
	if err != nil {
		return nil, fmt.Errorf("query latest metrics: %w", err)
	}
	defer rows.Close()

	summary := &CycleSummary{
		GeneratedAt: time.Now(),
		Assessments: make([]MetricAssessment, 0, 4),
	}

	for rows.Next() {
		var assessment MetricAssessment
		if err := rows.Scan(
			&assessment.MetricType,
			&assessment.Value,
			&assessment.Unit,
			&assessment.CollectedAt,
		); err != nil {
			return nil, fmt.Errorf("scan metric row: %w", err)
		}

		assessment.Severity = severityFor(assessment.MetricType, assessment.Value, assessment.Unit)
		summary.Assessments = append(summary.Assessments, assessment)

		summary.MetricsTotal++
		switch assessment.Severity {
		case SeverityCritical:
			summary.CriticalCount++
		case SeverityWarning:
			summary.WarningCount++
		}

		metricAge := time.Since(assessment.CollectedAt)
		if metricAge > summary.OldestMetricAge {
			summary.OldestMetricAge = metricAge
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate metric rows: %w", err)
	}

	return summary, nil
}

func severityFor(metricType string, value float64, unit string) Severity {
	switch metricType {
	case "cpu", "memory", "disk":
		if unit == "%" && value > 90.0 {
			return SeverityCritical
		}
		if unit == "%" && value > 75.0 {
			return SeverityWarning
		}
		return SeverityOK
	case "network":
		if unit == "MB/s" && value > 100.0 {
			return SeverityCritical
		}
		if unit == "MB/s" && value > 50.0 {
			return SeverityWarning
		}
		return SeverityOK
	default:
		return SeverityOK
	}
}
