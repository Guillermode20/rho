package codecore

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/earendil-works/rho/pkg/ai"
)

// FooterData represents the data displayed in the TUI footer/status bar.
type FooterData struct {
	GitBranch       string            `json:"gitBranch"`
	ModelName       string            `json:"modelName"`
	ProviderName    string            `json:"providerName"`
	TokenCount      int               `json:"tokenCount"`
	ContextWindow   int               `json:"contextWindow"`
	ContextPercent  float64           `json:"contextPercent"`
	ThinkingLevel   string            `json:"thinkingLevel"`
	ExtensionStatus map[string]string `json:"extensionStatus"`
	MessageCount    int               `json:"messageCount"`
	Cost            float64           `json:"cost"`
	IsStreaming     bool              `json:"isStreaming"`
	ErrorMessage    string            `json:"errorMessage,omitempty"`
	Version         string            `json:"version"`
	CWD             string            `json:"cwd"`
}

// FooterSegment represents a single segment in the footer display.
type FooterSegment struct {
	Text  string `json:"text"`
	Color string `json:"color,omitempty"`
	Bold  bool   `json:"bold,omitempty"`
}

// ReadonlyFooterDataProvider provides read-only access to footer data.
// Extensions cannot modify this directly; they use SetStatus on UIContext.
type ReadonlyFooterDataProvider struct {
	mu     sync.RWMutex
	data   FooterData
	labels map[string]string // extension status labels
}

// NewFooterDataProvider creates a new footer data provider.
func NewFooterDataProvider() *ReadonlyFooterDataProvider {
	return &ReadonlyFooterDataProvider{
		labels: make(map[string]string),
	}
}

// SetGitBranch sets the git branch name.
func (f *ReadonlyFooterDataProvider) SetGitBranch(branch string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data.GitBranch = branch
}

// SetModel sets the model and provider info.
func (f *ReadonlyFooterDataProvider) SetModel(model ai.Model) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data.ModelName = model.Name
	f.data.ProviderName = GetProviderShortName(model.Provider)
}

// SetTokenCount sets token usage info.
func (f *ReadonlyFooterDataProvider) SetTokenCount(count, window int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data.TokenCount = count
	f.data.ContextWindow = window
	if window > 0 {
		f.data.ContextPercent = float64(count) / float64(window) * 100
	}
}

// SetThinkingLevel sets the thinking level.
func (f *ReadonlyFooterDataProvider) SetThinkingLevel(level string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data.ThinkingLevel = level
}

// SetMessageCount sets the message count.
func (f *ReadonlyFooterDataProvider) SetMessageCount(count int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data.MessageCount = count
}

// SetStreaming sets the streaming status.
func (f *ReadonlyFooterDataProvider) SetStreaming(streaming bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data.IsStreaming = streaming
}

// SetCost sets the accumulated cost.
func (f *ReadonlyFooterDataProvider) SetCost(cost float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data.Cost = cost
}

// SetCWD sets the current working directory.
func (f *ReadonlyFooterDataProvider) SetCWD(cwd string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data.CWD = cwd
}

// SetError sets an error message.
func (f *ReadonlyFooterDataProvider) SetError(msg string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data.ErrorMessage = msg
}

// SetExtensionStatus sets a key/value status from an extension.
func (f *ReadonlyFooterDataProvider) SetExtensionStatus(key, value string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if value == "" {
		delete(f.labels, key)
	} else {
		f.labels[key] = value
	}
	f.data.ExtensionStatus = f.labels
}

// GetData returns a copy of the current footer data.
func (f *ReadonlyFooterDataProvider) GetData() FooterData {
	f.mu.RLock()
	defer f.mu.RUnlock()
	d := f.data
	d.ExtensionStatus = make(map[string]string)
	for k, v := range f.labels {
		d.ExtensionStatus[k] = v
	}
	return d
}

// GetSegments returns the footer as display segments.
func (f *ReadonlyFooterDataProvider) GetSegments() []FooterSegment {
	data := f.GetData()
	var segments []FooterSegment

	// Left side: model info
	modelLabel := data.ProviderName + "/" + data.ModelName
	if data.IsStreaming {
		modelLabel = "▶ " + modelLabel
	}
	segments = append(segments, FooterSegment{Text: modelLabel, Color: "cyan", Bold: true})

	// Git branch
	if data.GitBranch != "" {
		segments = append(segments, FooterSegment{Text: " ⎇ " + data.GitBranch, Color: "green"})
	}

	// Token usage
	if data.ContextWindow > 0 {
		pct := fmt.Sprintf("%.0f%%", data.ContextPercent)
		segments = append(segments, FooterSegment{Text: " ☰ " + pct, Color: "yellow"})
	}

	// Message count
	if data.MessageCount > 0 {
		segments = append(segments, FooterSegment{Text: fmt.Sprintf(" #%d", data.MessageCount), Color: "dim"})
	}

	// Thinking level
	if data.ThinkingLevel != "" && data.ThinkingLevel != "off" {
		segments = append(segments, FooterSegment{Text: " ⟐ " + data.ThinkingLevel, Color: "magenta"})
	}

	// Cost
	if data.Cost > 0 {
		segments = append(segments, FooterSegment{Text: fmt.Sprintf(" $%.4f", data.Cost), Color: "dim"})
	}

	// Extension statuses (sorted by key)
	if len(data.ExtensionStatus) > 0 {
		var keys []string
		for k := range data.ExtensionStatus {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := data.ExtensionStatus[k]
			segments = append(segments, FooterSegment{Text: " " + k + ":" + v, Color: "dim"})
		}
	}

	// Error message
	if data.ErrorMessage != "" {
		segments = append(segments, FooterSegment{Text: " ⚠ " + data.ErrorMessage, Color: "red", Bold: true})
	}

	return segments
}

// RenderText renders the footer as a plain text string with ANSI coloring.
func (f *ReadonlyFooterDataProvider) RenderText(width int) string {
	segments := f.GetSegments()
	var parts []string
	for _, seg := range segments {
		text := seg.Text
		parts = append(parts, text)
	}
	joined := strings.Join(parts, " │ ")
	if len(joined) > width {
		joined = joined[:width]
	}
	return joined
}

// RenderSegments renders the footer as colored segments for the TUI.
func (f *ReadonlyFooterDataProvider) RenderSegments() []FooterSegment {
	return f.GetSegments()
}
