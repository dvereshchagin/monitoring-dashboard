package routing

import "strings"

// Target defines an upstream destination name.
type Target string

const (
	TargetAPI      Target = "api"
	TargetAnalyzer Target = "analyzer"
)

// Match resolves incoming path to an upstream target.
func Match(path string) (Target, bool) {
	switch {
	case path == "/api/v1/release-analyzer" || strings.HasPrefix(path, "/api/v1/release-analyzer/"):
		return TargetAnalyzer, true
	case path == "/ws" || strings.HasPrefix(path, "/ws/"):
		return TargetAPI, true
	case path == "/api/v1" || strings.HasPrefix(path, "/api/v1/"):
		return TargetAPI, true
	case path == "/api" || strings.HasPrefix(path, "/api/"):
		return TargetAPI, true
	default:
		return "", false
	}
}
