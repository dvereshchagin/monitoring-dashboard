package releaseanalyzer

import "time"

type Severity string

const (
	SeverityOK       Severity = "ok"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type MetricAssessment struct {
	MetricType  string
	Value       float64
	Unit        string
	CollectedAt time.Time
	Severity    Severity
}

type CycleSummary struct {
	GeneratedAt     time.Time
	MetricsTotal    int
	CriticalCount   int
	WarningCount    int
	OldestMetricAge time.Duration
	Assessments     []MetricAssessment
}

type Snapshot struct {
	StartedAt   time.Time
	Interval    time.Duration
	LastRunAt   time.Time
	LastError   string
	LastSummary *CycleSummary
}
