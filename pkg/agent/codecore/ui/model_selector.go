package codecui

import (
	"fmt"
	"strings"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/ai"
	"github.com/earendil-works/rho/pkg/tui"
)

// ModelSelector is a dialog for selecting AI models with filtering and provider grouping.
type ModelSelector struct {
	models       []ai.Model
	providers    map[string][]ai.Model
	providerOrder []string
	selectedIdx  int
	filter       string
	focused      bool
	onSelect     func(model ai.Model)
	onCancel     func()
	showThinking bool
}

// NewModelSelector creates a new model selector dialog.
func NewModelSelector(models []ai.Model) *ModelSelector {
	providers := make(map[string][]ai.Model)
	var providerOrder []string
	for _, m := range models {
		key := string(m.Provider)
		if _, ok := providers[key]; !ok {
			providerOrder = append(providerOrder, key)
		}
		providers[key] = append(providers[key], m)
	}
	return &ModelSelector{
		models:        models,
		providers:     providers,
		providerOrder: providerOrder,
	}
}

// SetOnSelect sets the selection callback.
func (ms *ModelSelector) SetOnSelect(fn func(model ai.Model)) {
	ms.onSelect = fn
}

// SetOnCancel sets the cancel callback.
func (ms *ModelSelector) SetOnCancel(fn func()) {
	ms.onCancel = fn
}

// SetFilter sets the search filter.
func (ms *ModelSelector) SetFilter(filter string) {
	ms.filter = strings.ToLower(filter)
	ms.selectedIdx = 0
}

func (ms *ModelSelector) SetFocused(focused bool) {
	ms.focused = focused
}

func (ms *ModelSelector) Focused() bool {
	return ms.focused
}

// getFilteredModels returns models matching the current filter.
func (ms *ModelSelector) getFilteredModels() []ai.Model {
	if ms.filter == "" {
		return ms.models
	}
	var filtered []ai.Model
	for _, m := range ms.models {
		if strings.Contains(strings.ToLower(m.Name), ms.filter) ||
			strings.Contains(strings.ToLower(string(m.Provider)), ms.filter) {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

func (ms *ModelSelector) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	reset := "\x1b[0m"
	bold := "\x1b[1m"
	dim := "\x1b[2m"
	cyan := "\x1b[36m"
	yellow := "\x1b[33m"
_ = "\x1b[32m"

	var lines []string
	lines = append(lines, bold+cyan+"Model Selection"+reset)
	lines = append(lines, dim+"  Search: "+ms.filter+"_"+reset)
	lines = append(lines, "")

	filtered := ms.getFilteredModels()
	if len(filtered) == 0 {
		lines = append(lines, dim+"  No matching models"+reset)
		return lines
	}

	currentProvider := ""
	for i, m := range filtered {
		if ms.filter == "" && string(m.Provider) != currentProvider {
			currentProvider = string(m.Provider)
			lines = append(lines, bold+"\u2502 "+currentProvider+reset)
		}

		prefix := "  "
		if i == ms.selectedIdx && ms.focused {
			prefix = "\u203a " + cyan
		}

		reasoning := ""
		if m.Name == "o3-mini" || strings.Contains(m.Name, "sonnet-4") || strings.Contains(m.Name, "gemini-2.5") {
			reasoning = yellow + " [reasoning]" + reset
		}

		modelLine := fmt.Sprintf("%s%s%s%s", prefix, m.Name, dim+" ("+string(m.API)+")"+reset, reasoning)
		lines = append(lines, modelLine)
	}

	return lines
}

func (ms *ModelSelector) HandleInput(data string) {
	switch {
	case tui.MatchesKey(data, "up") || tui.MatchesKey(data, "ctrl+p"):
		if ms.selectedIdx > 0 {
			ms.selectedIdx--
		}
	case tui.MatchesKey(data, "down") || tui.MatchesKey(data, "ctrl+n"):
		filtered := ms.getFilteredModels()
		if ms.selectedIdx < len(filtered)-1 {
			ms.selectedIdx++
		}
	case tui.MatchesKey(data, "enter"):
		filtered := ms.getFilteredModels()
		if ms.selectedIdx < len(filtered) && ms.onSelect != nil {
			ms.onSelect(filtered[ms.selectedIdx])
		}
	case tui.MatchesKey(data, "escape"):
		if ms.onCancel != nil {
			ms.onCancel()
		}
	case tui.MatchesKey(data, "backspace"):
		if len(ms.filter) > 0 {
			ms.filter = ms.filter[:len(ms.filter)-1]
			ms.selectedIdx = 0
		}
	default:
		if len(data) == 1 && data[0] >= 0x20 && data[0] <= 0x7e {
			ms.filter += string(data[0])
			ms.selectedIdx = 0
		}
	}
}

func (ms *ModelSelector) Invalidate()           {}
func (ms *ModelSelector) WantsKeyRelease() bool { return false }

// ThinkingSelector selects thinking level for reasoning models.
type ThinkingSelector struct {
	levels       []ai.ThinkingLevel
	selectedIdx  int
	modelName    string
	focused      bool
	onSelect     func(level ai.ThinkingLevel)
	onCancel     func()
}

// NewThinkingSelector creates a thinking level selector.
func NewThinkingSelector(modelName string) *ThinkingSelector {
	return &ThinkingSelector{
		levels:   []ai.ThinkingLevel{ai.ThinkingMinimal, ai.ThinkingLow, ai.ThinkingMedium, ai.ThinkingHigh, ai.ThinkingXHigh},
		modelName: modelName,
	}
}

// SetOnSelect sets the selection callback.
func (ts *ThinkingSelector) SetOnSelect(fn func(level ai.ThinkingLevel)) {
	ts.onSelect = fn
}

// SetOnCancel sets the cancel callback.
func (ts *ThinkingSelector) SetOnCancel(fn func()) {
	ts.onCancel = fn
}

func (ts *ThinkingSelector) SetFocused(focused bool) {
	ts.focused = focused
}

func (ts *ThinkingSelector) Focused() bool {
	return ts.focused
}

func (ts *ThinkingSelector) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	reset := "\x1b[0m"
	bold := "\x1b[1m"
	dim := "\x1b[2m"
	cyan := "\x1b[36m"
	yellow := "\x1b[33m"

	var lines []string
	lines = append(lines, bold+cyan+"Thinking Level: "+ts.modelName+reset)
	lines = append(lines, dim+"  Select how much the model should reason"+reset)
	lines = append(lines, "")

	for i, level := range ts.levels {
		prefix := "  "
		if i == ts.selectedIdx && ts.focused {
			prefix = "\u203a " + cyan
		}

		levelName := string(level)
		levelDesc := ""
		switch level {
		case ai.ThinkingMinimal:
			levelDesc = "Minimal reasoning, faster responses"
		case ai.ThinkingLow:
			levelDesc = "Low reasoning effort"
		case ai.ThinkingMedium:
			levelDesc = "Moderate reasoning effort"
		case ai.ThinkingHigh:
			levelDesc = "High reasoning effort"
		case ai.ThinkingXHigh:
			levelDesc = "Maximum reasoning effort"
		}

		line := fmt.Sprintf("%s%s%s  %s%s", prefix, levelName, reset, dim+levelDesc+reset, yellow)
		lines = append(lines, line)
	}

	return lines
}

func (ts *ThinkingSelector) HandleInput(data string) {
	switch {
	case tui.MatchesKey(data, "up") || tui.MatchesKey(data, "ctrl+p"):
		if ts.selectedIdx > 0 {
			ts.selectedIdx--
		}
	case tui.MatchesKey(data, "down") || tui.MatchesKey(data, "ctrl+n"):
		if ts.selectedIdx < len(ts.levels)-1 {
			ts.selectedIdx++
		}
	case tui.MatchesKey(data, "enter"):
		if ts.selectedIdx < len(ts.levels) && ts.onSelect != nil {
			ts.onSelect(ts.levels[ts.selectedIdx])
		}
	case tui.MatchesKey(data, "escape"):
		if ts.onCancel != nil {
			ts.onCancel()
		}
	}
}

func (ts *ThinkingSelector) Invalidate()           {}
func (ts *ThinkingSelector) WantsKeyRelease() bool { return false }

// ShowImagesSelector toggles display of inline images.
type ShowImagesSelector struct {
	showImages bool
	onToggle   func(show bool)
	focused    bool
}

// NewShowImagesSelector creates a show-images selector.
func NewShowImagesSelector(initial bool) *ShowImagesSelector {
	return &ShowImagesSelector{showImages: initial}
}

// SetOnToggle sets the toggle callback.
func (sis *ShowImagesSelector) SetOnToggle(fn func(show bool)) {
	sis.onToggle = fn
}

func (sis *ShowImagesSelector) SetFocused(focused bool) {
	sis.focused = focused
}

func (sis *ShowImagesSelector) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	reset := "\x1b[0m"
	bold := "\x1b[1m"
_ = "\x1b[36m"
	green := "\x1b[32m"
	dim := "\x1b[2m"

	var lines []string
	prefix := "  "
	if sis.focused {
		prefix = "\u203a "
	}

	status := dim + "off" + reset
	if sis.showImages {
		status = green + "on" + reset
	}

	lines = append(lines, fmt.Sprintf("%s%sShow Images: %s%s", prefix, bold, status, reset))
	return lines
}

func (sis *ShowImagesSelector) HandleInput(data string) {
	if sis.focused && (tui.MatchesKey(data, "enter") || tui.MatchesKey(data, " ") || tui.MatchesKey(data, "left") || tui.MatchesKey(data, "right")) {
		sis.showImages = !sis.showImages
		if sis.onToggle != nil {
			sis.onToggle(sis.showImages)
		}
	}
}

func (sis *ShowImagesSelector) Invalidate()           {}
func (sis *ShowImagesSelector) WantsKeyRelease() bool { return false }

// Ensure types are used.
var _ = agent.AgentLoopConfig{}
