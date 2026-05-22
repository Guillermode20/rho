package codecore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TelemetryEvent describes a single telemetry event.
type TelemetryEvent struct {
	Type      string                 `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	SessionID string                 `json:"sessionId,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// TelemetryCollector collects telemetry events with user opt-in/opt-out.
type TelemetryCollector struct {
	mu         sync.RWMutex
	enabled    bool
	filePath   string
	events     []TelemetryEvent
	sessionID  string
	startTime  time.Time
	eventCount int
}

// NewTelemetryCollector creates a new telemetry collector.
// Telemetry is opt-in and disabled by default.
func NewTelemetryCollector(rhoDir string) *TelemetryCollector {
	tc := &TelemetryCollector{
		enabled:   false,
		filePath:  filepath.Join(rhoDir, "telemetry.jsonl"),
		startTime: time.Now(),
	}

	// Check if opt-in file exists
	optInPath := filepath.Join(rhoDir, ".telemetry-opt-in")
	if _, err := os.Stat(optInPath); err == nil {
		tc.enabled = true
	}

	return tc
}

// SetEnabled enables or disables telemetry.
func (tc *TelemetryCollector) SetEnabled(enabled bool) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.enabled = enabled

	// Create or remove opt-in marker
	rhoDir := filepath.Dir(tc.filePath)
	optInPath := filepath.Join(rhoDir, ".telemetry-opt-in")
	if enabled {
		os.MkdirAll(rhoDir, 0755)
		os.WriteFile(optInPath, []byte("1"), 0644)
	} else {
		os.Remove(optInPath)
	}
}

// IsEnabled returns whether telemetry is enabled.
func (tc *TelemetryCollector) IsEnabled() bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.enabled
}

// SetSessionID sets the current session ID for event correlation.
func (tc *TelemetryCollector) SetSessionID(id string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.sessionID = id
}

// Record records a telemetry event (no-op if disabled).
func (tc *TelemetryCollector) Record(eventType string, data map[string]interface{}) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if !tc.enabled {
		return
	}

	event := TelemetryEvent{
		Type:      eventType,
		Timestamp: time.Now().UnixMilli(),
		SessionID: tc.sessionID,
		Data:      data,
	}

	tc.events = append(tc.events, event)
	tc.eventCount++

	// Periodically flush to disk
	if len(tc.events) >= 10 {
		tc.flush()
	}
}

// RecordEvent records an event with structured data.
func (tc *TelemetryCollector) RecordEvent(event TelemetryEvent) {
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixMilli()
	}
	if event.SessionID == "" {
		event.SessionID = tc.sessionID
	}
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if !tc.enabled {
		return
	}
	tc.events = append(tc.events, event)
	tc.eventCount++
	if len(tc.events) >= 10 {
		tc.flush()
	}
}

// RecordSessionMetrics records session-level metrics.
func (tc *TelemetryCollector) RecordSessionMetrics(messageCount int, toolCallCount int, inputTokens, outputTokens int) {
	tc.Record("session_metrics", map[string]interface{}{
		"messageCount":  messageCount,
		"toolCallCount": toolCallCount,
		"inputTokens":   inputTokens,
		"outputTokens":  outputTokens,
		"durationMs":    time.Since(tc.startTime).Milliseconds(),
	})
}

// RecordModelUsage records a model invocation.
func (tc *TelemetryCollector) RecordModelUsage(provider, model string, inputTokens, outputTokens int) {
	tc.Record("model_usage", map[string]interface{}{
		"provider":     provider,
		"model":        model,
		"inputTokens":  inputTokens,
		"outputTokens": outputTokens,
	})
}

// RecordToolExecution records a tool execution.
func (tc *TelemetryCollector) RecordToolExecution(toolName string, success bool, durationMs int64) {
	tc.Record("tool_execution", map[string]interface{}{
		"tool":       toolName,
		"success":    success,
		"durationMs": durationMs,
	})
}

// Flush forces pending events to be written to disk.
func (tc *TelemetryCollector) Flush() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.flush()
}

func (tc *TelemetryCollector) flush() {
	if len(tc.events) == 0 {
		return
	}

	// Append events to JSONL file
	f, err := os.OpenFile(tc.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	for _, event := range tc.events {
		data, err := json.Marshal(event)
		if err == nil {
			f.Write(data)
			f.WriteString("\n")
		}
	}

	tc.events = tc.events[:0]
}

// GetStats returns current telemetry statistics.
func (tc *TelemetryCollector) GetStats() TelemetryStats {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return TelemetryStats{
		Enabled:      tc.enabled,
		EventCount:   tc.eventCount,
		SessionID:    tc.sessionID,
		UptimeMs:     time.Since(tc.startTime).Milliseconds(),
		PendingFlush: len(tc.events),
	}
}

// TelemetryStats provides a snapshot of telemetry state.
type TelemetryStats struct {
	Enabled      bool   `json:"enabled"`
	EventCount   int    `json:"eventCount"`
	SessionID    string `json:"sessionId"`
	UptimeMs     int64  `json:"uptimeMs"`
	PendingFlush int    `json:"pendingFlush"`
}

// RecordError records an error event.
func (tc *TelemetryCollector) RecordError(context string, err error) {
	tc.Record("error", map[string]interface{}{
		"context": context,
		"error":   err.Error(),
	})
}

// RecordStartup records a startup event.
func (tc *TelemetryCollector) RecordStartup(version string) {
	tc.Record("startup", map[string]interface{}{
		"version": version,
	})
}
