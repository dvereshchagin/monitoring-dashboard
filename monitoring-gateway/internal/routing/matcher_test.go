package routing

import "testing"

func TestMatch(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantOK     bool
		wantTarget Target
	}{
		{name: "analyzer route", path: "/api/v1/release-analyzer/summary", wantOK: true, wantTarget: TargetAnalyzer},
		{name: "websocket route", path: "/ws", wantOK: true, wantTarget: TargetAPI},
		{name: "v1 api route", path: "/api/v1/metrics/history", wantOK: true, wantTarget: TargetAPI},
		{name: "legacy api route", path: "/api/metrics/history", wantOK: true, wantTarget: TargetAPI},
		{name: "ui route", path: "/", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, ok := Match(tt.path)
			if ok != tt.wantOK {
				t.Fatalf("Match() ok = %v, want %v", ok, tt.wantOK)
			}
			if target != tt.wantTarget {
				t.Fatalf("Match() target = %q, want %q", target, tt.wantTarget)
			}
		})
	}
}
