package aiutils

import "fmt"

// DiagnosticSeverity indicates the severity of a diagnostic message.
type DiagnosticSeverity string

const (
	DiagInfo    DiagnosticSeverity = "info"
	DiagWarning DiagnosticSeverity = "warning"
	DiagError   DiagnosticSeverity = "error"
)

// Diagnostic represents a provider or runtime diagnostic message.
type Diagnostic struct {
	Severity DiagnosticSeverity `json:"severity"`
	Message  string             `json:"message"`
	Code     string             `json:"code,omitempty"`
	Source   string             `json:"source,omitempty"` // e.g., "anthropic", "openai", "compaction"
}

// DiagnosticCollector collects and manages diagnostic messages.
type DiagnosticCollector struct {
	diagnostics []Diagnostic
	maxSize     int
}

// NewDiagnosticCollector creates a new diagnostic collector.
func NewDiagnosticCollector(maxSize int) *DiagnosticCollector {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &DiagnosticCollector{
		maxSize: maxSize,
	}
}

// Add adds a diagnostic message.
func (dc *DiagnosticCollector) Add(severity DiagnosticSeverity, message, code, source string) {
	if len(dc.diagnostics) >= dc.maxSize {
		dc.diagnostics = dc.diagnostics[1:]
	}
	dc.diagnostics = append(dc.diagnostics, Diagnostic{
		Severity: severity,
		Message:  message,
		Code:     code,
		Source:   source,
	})
}

// AddWarning adds a warning diagnostic.
func (dc *DiagnosticCollector) AddWarning(message, code, source string) {
	dc.Add(DiagWarning, message, code, source)
}

// AddError adds an error diagnostic.
func (dc *DiagnosticCollector) AddError(message, code, source string) {
	dc.Add(DiagError, message, code, source)
}

// AddInfo adds an info diagnostic.
func (dc *DiagnosticCollector) AddInfo(message, code, source string) {
	dc.Add(DiagInfo, message, code, source)
}

// GetAll returns all collected diagnostics.
func (dc *DiagnosticCollector) GetAll() []Diagnostic {
	result := make([]Diagnostic, len(dc.diagnostics))
	copy(result, dc.diagnostics)
	return result
}

// Drain returns all diagnostics and clears the collection.
func (dc *DiagnosticCollector) Drain() []Diagnostic {
	result := dc.GetAll()
	dc.diagnostics = nil
	return result
}

// HasErrors returns true if any errors have been collected.
func (dc *DiagnosticCollector) HasErrors() bool {
	for _, d := range dc.diagnostics {
		if d.Severity == DiagError {
			return true
		}
	}
	return false
}

// FormatDiagnostic formats a single diagnostic for display.
func FormatDiagnostic(d Diagnostic) string {
	prefix := ""
	switch d.Severity {
	case DiagError:
		prefix = "Error"
	case DiagWarning:
		prefix = "Warning"
	case DiagInfo:
		prefix = "Info"
	}
	result := prefix + ": " + d.Message
	if d.Code != "" {
		result += " [" + d.Code + "]"
	}
	if d.Source != "" {
		result += " (" + d.Source + ")"
	}
	return result
}

// FormatDiagnostics formats all diagnostics for display.
func FormatDiagnostics(diags []Diagnostic) []string {
	result := make([]string, len(diags))
	for i, d := range diags {
		result[i] = FormatDiagnostic(d)
	}
	return result
}

// ProviderError wraps provider errors with diagnostics.
type ProviderError struct {
	Provider    string `json:"provider"`
	StatusCode  int    `json:"statusCode"`
	Message     string `json:"message"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

func (e *ProviderError) Error() string {
	return e.Provider + " (status " + fmt.Sprintf("%d", e.StatusCode) + "): " + e.Message
}
