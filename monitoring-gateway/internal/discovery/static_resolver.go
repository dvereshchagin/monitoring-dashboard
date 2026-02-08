package discovery

import (
	"context"
	"fmt"
	"net/url"
)

// StaticResolver keeps fixed upstream URLs for non-cluster environments.
type StaticResolver struct {
	apiURL      *url.URL
	analyzerURL *url.URL
}

func NewStaticResolver(apiURL, analyzerURL string) (*StaticResolver, error) {
	parsedAPI, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("parse API_UPSTREAM_URL: %w", err)
	}
	parsedAnalyzer, err := url.Parse(analyzerURL)
	if err != nil {
		return nil, fmt.Errorf("parse ANALYZER_UPSTREAM_URL: %w", err)
	}

	return &StaticResolver{
		apiURL:      parsedAPI,
		analyzerURL: parsedAnalyzer,
	}, nil
}

func (r *StaticResolver) Resolve(_ context.Context) (Snapshot, error) {
	return Snapshot{
		APIURL:      r.apiURL,
		AnalyzerURL: r.analyzerURL,
	}, nil
}
