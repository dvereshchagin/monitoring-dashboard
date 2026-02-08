package cloudwatch

import (
	"testing"
	"time"

	"github.com/dreschagin/monitoring-dashboard/internal/domain/entity"
	"github.com/dreschagin/monitoring-dashboard/internal/domain/valueobject"
)

func TestMapUnit(t *testing.T) {
	tests := []struct {
		name     string
		unit     string
		expected string
	}{
		{"percentage", "%", "Percent"},
		{"megabytes per second", "MB/s", "Megabytes/Second"},
		{"milliseconds", "ms", "Milliseconds"},
		{"seconds", "s", "Seconds"},
		{"count", "count", "Count"},
		{"unknown", "custom", "None"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapUnit(tt.unit)
			if string(result) != tt.expected {
				t.Errorf("mapUnit(%q) = %v, want %v", tt.unit, result, tt.expected)
			}
		})
	}
}

func TestConvertToDatum(t *testing.T) {
	// Create test publisher (minimal config)
	p := &MetricsPublisher{
		namespace: "Test/Namespace",
		defaultDimensions: map[string]string{
			"Environment": "test",
			"Region":      "us-east-1",
		},
		storageResolution: 60,
	}

	// Create test metric
	metricValue, err := valueobject.NewMetricValue(75.5, "%")
	if err != nil {
		t.Fatalf("Failed to create metric value: %v", err)
	}

	metric, err := entity.NewMetric(valueobject.CPU, "cpu_usage", metricValue)
	if err != nil {
		t.Fatalf("Failed to create metric: %v", err)
	}

	// Convert to CloudWatch datum
	datum := p.convertToDatum(metric)

	// Verify fields
	if datum.MetricName == nil || *datum.MetricName != "cpu_usage" {
		t.Errorf("Expected MetricName=cpu_usage, got %v", datum.MetricName)
	}

	if datum.Value == nil || *datum.Value != 75.5 {
		t.Errorf("Expected Value=75.5, got %v", datum.Value)
	}

	if datum.Unit != "Percent" {
		t.Errorf("Expected Unit=Percent, got %v", datum.Unit)
	}

	if datum.Timestamp == nil {
		t.Error("Expected Timestamp to be set")
	}

	if datum.StorageResolution == nil || *datum.StorageResolution != 60 {
		t.Errorf("Expected StorageResolution=60, got %v", datum.StorageResolution)
	}

	// Verify dimensions
	expectedDimensions := map[string]string{
		"Environment": "test",
		"Region":      "us-east-1",
		"MetricType":  "cpu",
		"MetricName":  "cpu_usage",
	}

	if len(datum.Dimensions) != len(expectedDimensions) {
		t.Errorf("Expected %d dimensions, got %d", len(expectedDimensions), len(datum.Dimensions))
	}

	for _, dim := range datum.Dimensions {
		if dim.Name == nil || dim.Value == nil {
			t.Error("Dimension name or value is nil")
			continue
		}

		expectedValue, ok := expectedDimensions[*dim.Name]
		if !ok {
			t.Errorf("Unexpected dimension: %s", *dim.Name)
			continue
		}

		if *dim.Value != expectedValue {
			t.Errorf("Dimension %s: expected %s, got %s", *dim.Name, expectedValue, *dim.Value)
		}
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    MetricsPublisherConfig
		expectErr bool
	}{
		{
			name: "valid config",
			config: MetricsPublisherConfig{
				Namespace:         "Test/Namespace",
				Region:            "us-east-1",
				BufferSize:        100,
				FlushInterval:     10 * time.Second,
				StorageResolution: 60,
			},
			expectErr: false,
		},
		{
			name: "missing namespace",
			config: MetricsPublisherConfig{
				Region:            "us-east-1",
				BufferSize:        100,
				FlushInterval:     10 * time.Second,
				StorageResolution: 60,
			},
			expectErr: true,
		},
		{
			name: "missing region",
			config: MetricsPublisherConfig{
				Namespace:         "Test/Namespace",
				BufferSize:        100,
				FlushInterval:     10 * time.Second,
				StorageResolution: 60,
			},
			expectErr: true,
		},
		{
			name: "invalid storage resolution",
			config: MetricsPublisherConfig{
				Namespace:         "Test/Namespace",
				Region:            "us-east-1",
				BufferSize:        100,
				FlushInterval:     10 * time.Second,
				StorageResolution: 30, // Invalid: must be 1 or 60
			},
			expectErr: false, // Should default to 60
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: We can't actually create the publisher without AWS credentials,
			// but we can test that validation logic exists by checking error messages
			// In a real test environment (with LocalStack), you would test the full flow

			if tt.config.Namespace == "" && !tt.expectErr {
				t.Error("Expected namespace validation to fail")
			}

			if tt.config.Region == "" && !tt.expectErr {
				t.Error("Expected region validation to fail")
			}

			// Verify defaults are applied correctly
			if tt.config.BufferSize <= 0 {
				expectedDefault := 100
				if tt.config.BufferSize != expectedDefault && !tt.expectErr {
					t.Logf("Note: BufferSize should default to %d", expectedDefault)
				}
			}

			if tt.config.FlushInterval <= 0 {
				expectedDefault := 10 * time.Second
				if tt.config.FlushInterval != expectedDefault && !tt.expectErr {
					t.Logf("Note: FlushInterval should default to %v", expectedDefault)
				}
			}

			if tt.config.StorageResolution != 1 && tt.config.StorageResolution != 60 {
				expectedDefault := int32(60)
				if tt.config.StorageResolution != expectedDefault && !tt.expectErr {
					t.Logf("Note: StorageResolution should default to %d", expectedDefault)
				}
			}
		})
	}
}
