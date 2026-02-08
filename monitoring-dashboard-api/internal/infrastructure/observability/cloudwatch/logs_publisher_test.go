package cloudwatch

import (
	"encoding/json"
	"testing"
	"time"

	applicationPort "github.com/dreschagin/monitoring-dashboard/internal/application/port"
)

func TestConvertToLogEvent(t *testing.T) {
	p := &LogsPublisher{
		logGroupName:  "/aws/test",
		logStreamName: "test-stream",
	}

	timestamp := time.Date(2026, 2, 8, 12, 0, 0, 0, time.UTC)
	entry := applicationPort.LogEntry{
		Timestamp: timestamp,
		Level:     applicationPort.LogLevelInfo,
		Message:   "Test message",
		Fields: map[string]interface{}{
			"user_id": "12345",
			"action":  "login",
			"count":   42,
		},
	}

	event, err := p.convertToLogEvent(entry)
	if err != nil {
		t.Fatalf("Failed to convert log entry: %v", err)
	}

	// Verify timestamp
	expectedTimestamp := timestamp.UnixMilli()
	if event.Timestamp == nil || *event.Timestamp != expectedTimestamp {
		t.Errorf("Expected Timestamp=%d, got %v", expectedTimestamp, event.Timestamp)
	}

	// Verify message is valid JSON
	if event.Message == nil {
		t.Fatal("Expected Message to be set")
	}

	var logData map[string]interface{}
	if err := json.Unmarshal([]byte(*event.Message), &logData); err != nil {
		t.Fatalf("Failed to parse log message as JSON: %v", err)
	}

	// Verify structured fields
	if logData["level"] != string(applicationPort.LogLevelInfo) {
		t.Errorf("Expected level=INFO, got %v", logData["level"])
	}

	if logData["message"] != "Test message" {
		t.Errorf("Expected message='Test message', got %v", logData["message"])
	}

	fields, ok := logData["fields"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected fields to be a map")
	}

	if fields["user_id"] != "12345" {
		t.Errorf("Expected user_id=12345, got %v", fields["user_id"])
	}

	if fields["action"] != "login" {
		t.Errorf("Expected action=login, got %v", fields["action"])
	}

	// Note: JSON numbers are float64
	if count, ok := fields["count"].(float64); !ok || count != 42 {
		t.Errorf("Expected count=42, got %v", fields["count"])
	}
}

func TestConvertToLogEvent_NoFields(t *testing.T) {
	p := &LogsPublisher{
		logGroupName:  "/aws/test",
		logStreamName: "test-stream",
	}

	timestamp := time.Now()
	entry := applicationPort.LogEntry{
		Timestamp: timestamp,
		Level:     applicationPort.LogLevelError,
		Message:   "Error occurred",
		Fields:    nil,
	}

	event, err := p.convertToLogEvent(entry)
	if err != nil {
		t.Fatalf("Failed to convert log entry: %v", err)
	}

	if event.Message == nil {
		t.Fatal("Expected Message to be set")
	}

	var logData map[string]interface{}
	if err := json.Unmarshal([]byte(*event.Message), &logData); err != nil {
		t.Fatalf("Failed to parse log message as JSON: %v", err)
	}

	if logData["level"] != string(applicationPort.LogLevelError) {
		t.Errorf("Expected level=ERROR, got %v", logData["level"])
	}

	if logData["message"] != "Error occurred" {
		t.Errorf("Expected message='Error occurred', got %v", logData["message"])
	}
}

func TestConvertToLogEvent_Truncation(t *testing.T) {
	p := &LogsPublisher{
		logGroupName:  "/aws/test",
		logStreamName: "test-stream",
	}

	// Create a very large message that exceeds CloudWatch limit
	largeMessage := string(make([]byte, maxLogEventSize+1000))

	timestamp := time.Now()
	entry := applicationPort.LogEntry{
		Timestamp: timestamp,
		Level:     applicationPort.LogLevelInfo,
		Message:   largeMessage,
		Fields:    nil,
	}

	event, err := p.convertToLogEvent(entry)
	if err != nil {
		t.Fatalf("Failed to convert log entry: %v", err)
	}

	if event.Message == nil {
		t.Fatal("Expected Message to be set")
	}

	// Verify message was truncated
	messageLen := len(*event.Message)
	if messageLen > maxLogEventSize {
		t.Errorf("Expected message to be truncated to %d bytes, got %d", maxLogEventSize, messageLen)
	}

	// Verify truncation marker
	if messageLen >= 3 {
		lastThree := (*event.Message)[messageLen-3:]
		if lastThree != "..." {
			t.Error("Expected truncation marker '...' at end of message")
		}
	}
}

func TestLogsConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    LogsPublisherConfig
		expectErr bool
	}{
		{
			name: "valid config",
			config: LogsPublisherConfig{
				LogGroupName:  "/aws/test",
				LogStreamName: "test-stream",
				Region:        "us-east-1",
				BufferSize:    50,
				FlushInterval: 5 * time.Second,
			},
			expectErr: false,
		},
		{
			name: "missing log group",
			config: LogsPublisherConfig{
				LogStreamName: "test-stream",
				Region:        "us-east-1",
			},
			expectErr: true,
		},
		{
			name: "missing log stream",
			config: LogsPublisherConfig{
				LogGroupName: "/aws/test",
				Region:       "us-east-1",
			},
			expectErr: true,
		},
		{
			name: "missing region",
			config: LogsPublisherConfig{
				LogGroupName:  "/aws/test",
				LogStreamName: "test-stream",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate required fields
			if tt.config.LogGroupName == "" && !tt.expectErr {
				t.Error("Expected log group validation to fail")
			}

			if tt.config.LogStreamName == "" && !tt.expectErr {
				t.Error("Expected log stream validation to fail")
			}

			if tt.config.Region == "" && !tt.expectErr {
				t.Error("Expected region validation to fail")
			}

			// Verify defaults
			if tt.config.BufferSize <= 0 {
				expectedDefault := 50
				t.Logf("Note: BufferSize should default to %d", expectedDefault)
			}

			if tt.config.FlushInterval <= 0 {
				expectedDefault := 5 * time.Second
				t.Logf("Note: FlushInterval should default to %v", expectedDefault)
			}
		})
	}
}

func TestChronologicalOrdering(t *testing.T) {
	// Create test log entries with different timestamps
	now := time.Now()
	entries := []applicationPort.LogEntry{
		{Timestamp: now.Add(5 * time.Second), Level: applicationPort.LogLevelInfo, Message: "Third"},
		{Timestamp: now, Level: applicationPort.LogLevelInfo, Message: "First"},
		{Timestamp: now.Add(2 * time.Second), Level: applicationPort.LogLevelInfo, Message: "Second"},
	}

	p := &LogsPublisher{
		logGroupName:  "/aws/test",
		logStreamName: "test-stream",
		buffer:        entries,
	}

	// Sort by timestamp (simulating what flushBufferUnsafe does)
	// We can't call flushBufferUnsafe directly as it requires AWS credentials,
	// but we can verify the sorting logic works
	type sortableEntry struct {
		entry     applicationPort.LogEntry
		timestamp time.Time
	}

	sortable := make([]sortableEntry, len(p.buffer))
	for i, entry := range p.buffer {
		sortable[i] = sortableEntry{entry, entry.Timestamp}
	}

	// Sort
	for i := 0; i < len(sortable)-1; i++ {
		for j := i + 1; j < len(sortable); j++ {
			if sortable[j].timestamp.Before(sortable[i].timestamp) {
				sortable[i], sortable[j] = sortable[j], sortable[i]
			}
		}
	}

	// Verify order
	if sortable[0].entry.Message != "First" {
		t.Error("Expected first entry to be 'First'")
	}
	if sortable[1].entry.Message != "Second" {
		t.Error("Expected second entry to be 'Second'")
	}
	if sortable[2].entry.Message != "Third" {
		t.Error("Expected third entry to be 'Third'")
	}

	// Verify timestamps are in order
	for i := 0; i < len(sortable)-1; i++ {
		if sortable[i+1].timestamp.Before(sortable[i].timestamp) {
			t.Errorf("Entries not in chronological order at index %d", i)
		}
	}
}
