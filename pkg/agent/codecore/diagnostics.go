package codecore

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// DiagnosticType represents the severity of a diagnostic.
type DiagnosticType string

const (
	DiagnosticTypeError   DiagnosticType = "error"
	DiagnosticTypeWarning DiagnosticType = "warning"
	DiagnosticTypeInfo    DiagnosticType = "info"
)

// Diagnostic represents a single diagnostic message.
type Diagnostic struct {
	Type    DiagnosticType `json:"type"`
	Message string         `json:"message"`
	Scope   string         `json:"scope,omitempty"`   // e.g., "settings", "extensions", "auth"
	Code    string         `json:"code,omitempty"`    // Machine-readable error code
	Details string         `json:"details,omitempty"` // Additional details
}

// DiagnosticsCollector collects and manages diagnostic messages.
type DiagnosticsCollector struct {
	mu          sync.RWMutex
	diagnostics []*Diagnostic
}

// NewDiagnosticsCollector creates a new diagnostics collector.
func NewDiagnosticsCollector() *DiagnosticsCollector {
	return &DiagnosticsCollector{}
}

// Report adds a diagnostic message.
func (dc *DiagnosticsCollector) Report(diag *Diagnostic) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.diagnostics = append(dc.diagnostics, diag)
}

// ReportError reports an error diagnostic.
func (dc *DiagnosticsCollector) ReportError(scope, message string, details ...string) {
	d := &Diagnostic{
		Type:    DiagnosticTypeError,
		Scope:   scope,
		Message: message,
	}
	if len(details) > 0 {
		d.Details = details[0]
	}
	dc.Report(d)
}

// ReportWarning reports a warning diagnostic.
func (dc *DiagnosticsCollector) ReportWarning(scope, message string, details ...string) {
	d := &Diagnostic{
		Type:    DiagnosticTypeWarning,
		Scope:   scope,
		Message: message,
	}
	if len(details) > 0 {
		d.Details = details[0]
	}
	dc.Report(d)
}

// ReportInfo reports an info diagnostic.
func (dc *DiagnosticsCollector) ReportInfo(scope, message string, details ...string) {
	d := &Diagnostic{
		Type:    DiagnosticTypeInfo,
		Scope:   scope,
		Message: message,
	}
	if len(details) > 0 {
		d.Details = details[0]
	}
	dc.Report(d)
}

// Drain returns all collected diagnostics and clears the collector.
func (dc *DiagnosticsCollector) Drain() []*Diagnostic {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	result := dc.diagnostics
	dc.diagnostics = nil
	return result
}

// GetAll returns all collected diagnostics without clearing.
func (dc *DiagnosticsCollector) GetAll() []*Diagnostic {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	result := make([]*Diagnostic, len(dc.diagnostics))
	copy(result, dc.diagnostics)
	return result
}

// HasErrors returns true if any error diagnostics exist.
func (dc *DiagnosticsCollector) HasErrors() bool {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	for _, d := range dc.diagnostics {
		if d.Type == DiagnosticTypeError {
			return true
		}
	}
	return false
}

// Count returns the number of diagnostics of a given type.
func (dc *DiagnosticsCollector) Count(diagType DiagnosticType) int {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	count := 0
	for _, d := range dc.diagnostics {
		if d.Type == diagType {
			count++
		}
	}
	return count
}

// Clear removes all diagnostics.
func (dc *DiagnosticsCollector) Clear() {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.diagnostics = nil
}

// FormatDiagnostic formats a single diagnostic for display.
func FormatDiagnostic(d *Diagnostic) string {
	var prefix string
	switch d.Type {
	case DiagnosticTypeError:
		prefix = "Error"
	case DiagnosticTypeWarning:
		prefix = "Warning"
	case DiagnosticTypeInfo:
		prefix = "Info"
	}

	parts := []string{fmt.Sprintf("%s: %s", prefix, d.Message)}
	if d.Scope != "" {
		parts = append([]string{fmt.Sprintf("[%s]", d.Scope)}, parts...)
	}
	if d.Details != "" {
		parts = append(parts, fmt.Sprintf("(%s)", d.Details))
	}

	return strings.Join(parts, " ")
}

// FormatDiagnostics formats all diagnostics for display.
func FormatDiagnostics(diags []*Diagnostic) string {
	if len(diags) == 0 {
		return ""
	}

	var lines []string
	for _, d := range diags {
		lines = append(lines, FormatDiagnostic(d))
	}

	// Count by type
	errorCount := 0
	warningCount := 0
	infoCount := 0
	for _, d := range diags {
		switch d.Type {
		case DiagnosticTypeError:
			errorCount++
		case DiagnosticTypeWarning:
			warningCount++
		case DiagnosticTypeInfo:
			infoCount++
		}
	}

	summary := fmt.Sprintf("%d diagnostic(s): %d error(s), %d warning(s), %d info",
		len(diags), errorCount, warningCount, infoCount)

	lines = append(lines, "", summary)
	return strings.Join(lines, "\n")
}

// SortDiagnostics sorts diagnostics by type (errors first, then warnings, then info).
func SortDiagnostics(diags []*Diagnostic) {
	sort.Slice(diags, func(i, j int) bool {
		order := map[DiagnosticType]int{
			DiagnosticTypeError:   0,
			DiagnosticTypeWarning: 1,
			DiagnosticTypeInfo:    2,
		}
		oi := order[diags[i].Type]
		oj := order[diags[j].Type]
		if oi != oj {
			return oi < oj
		}
		return diags[i].Message < diags[j].Message
	})
}

// ============================================================================
// Convenience functions
// ============================================================================

// DrainErrors drains all error diagnostics from a collector.
func DrainErrors(dc *DiagnosticsCollector) []*Diagnostic {
	all := dc.Drain()
	var errors []*Diagnostic
	for _, d := range all {
		if d.Type == DiagnosticTypeError {
			errors = append(errors, d)
		}
	}
	return errors
}

// DrainWarnings drains all warning diagnostics from a collector.
func DrainWarnings(dc *DiagnosticsCollector) []*Diagnostic {
	all := dc.Drain()
	var warnings []*Diagnostic
	for _, d := range all {
		if d.Type == DiagnosticTypeWarning {
			warnings = append(warnings, d)
		}
	}
	return warnings
}
