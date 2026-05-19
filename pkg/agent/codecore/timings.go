package codecore

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// TimingRecord stores a single timing measurement.
type TimingRecord struct {
	Label     string        `json:"label"`
	Duration  time.Duration `json:"duration"`
	StartTime time.Time     `json:"startTime"`
}

// TimeTracker tracks timing measurements.
type TimeTracker struct {
	mu      sync.Mutex
	records map[string][]TimingRecord
	starts  map[string]time.Time
}

var (
	globalTracker = NewTimeTracker()
)

// NewTimeTracker creates a new TimeTracker.
func NewTimeTracker() *TimeTracker {
	return &TimeTracker{
		records: make(map[string][]TimingRecord),
		starts:  make(map[string]time.Time),
	}
}

// Start begins timing a labeled operation.
func (t *TimeTracker) Start(label string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.starts[label] = time.Now()
}

// Stop ends timing a labeled operation and records the duration.
func (t *TimeTracker) Stop(label string) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()

	start, ok := t.starts[label]
	if !ok {
		return 0
	}

	duration := time.Since(start)
	t.records[label] = append(t.records[label], TimingRecord{
		Label:     label,
		Duration:  duration,
		StartTime: start,
	})
	delete(t.starts, label)
	return duration
}

// Record records a timing directly.
func (t *TimeTracker) Record(label string, duration time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records[label] = append(t.records[label], TimingRecord{
		Label:     label,
		Duration:  duration,
		StartTime: time.Now(),
	})
}

// GetRecords returns all records for a label.
func (t *TimeTracker) GetRecords(label string) []TimingRecord {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.records[label]
}

// GetAllLabels returns all tracked labels.
func (t *TimeTracker) GetAllLabels() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var labels []string
	for label := range t.records {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	return labels
}

// TotalDuration returns the total duration for a label.
func (t *TimeTracker) TotalDuration(label string) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	var total time.Duration
	for _, r := range t.records[label] {
		total += r.Duration
	}
	return total
}

// Reset clears all timing records.
func (t *TimeTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records = make(map[string][]TimingRecord)
	t.starts = make(map[string]time.Time)
}

// PrintTimings prints a timing summary.
func (t *TimeTracker) PrintTimings() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.records) == 0 {
		return "No timing records."
	}

	var lines []string
	lines = append(lines, "Timing Summary:")
	lines = append(lines, strings.Repeat("-", 60))

	var labels []string
	for label := range t.records {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	var grandTotal time.Duration
	for _, label := range labels {
		records := t.records[label]
		var total time.Duration
		count := len(records)
		for _, r := range records {
			total += r.Duration
		}
		grandTotal += total
		avg := total / time.Duration(count)
		lines = append(lines, fmt.Sprintf("  %-30s %8s  (count: %d, avg: %s)", label, total.Round(time.Millisecond), count, avg.Round(time.Millisecond)))
	}

	lines = append(lines, strings.Repeat("-", 60))
	lines = append(lines, fmt.Sprintf("  %-30s %8s", "Total", grandTotal.Round(time.Millisecond)))

	return strings.Join(lines, "\n")
}

// ============================================================================
// Package-Level Convenience Functions
// ============================================================================

// Time starts timing a labeled operation using the global tracker.
func Time(label string) {
	globalTracker.Start(label)
}

// TimeEnd stops timing a labeled operation using the global tracker.
func TimeEnd(label string) time.Duration {
	return globalTracker.Stop(label)
}

// PrintTimings returns a timing summary from the global tracker.
func PrintTimings() string {
	return globalTracker.PrintTimings()
}

// ResetTimings resets all timing records in the global tracker.
func ResetTimings() {
	globalTracker.Reset()
}

// TimeFunc times a function execution.
func TimeFunc(label string, fn func()) time.Duration {
	start := time.Now()
	fn()
	duration := time.Since(start)
	globalTracker.Record(label, duration)
	return duration
}

// TimeFuncResult times a function that returns a result.
func TimeFuncResult[T any](label string, fn func() T) (T, time.Duration) {
	start := time.Now()
	result := fn()
	duration := time.Since(start)
	globalTracker.Record(label, duration)
	return result, duration
}
