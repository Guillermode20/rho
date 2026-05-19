package codecui

import (
	"fmt"

	"github.com/earendil-works/rho/pkg/tui"
)

// ThemeInfo describes an available theme.
type ThemeInfo struct {
	Name        string
	Path        string
	IsBuiltin   bool
	IsCurrent   bool
	Description string
}

// ThemeSelector is a dialog for selecting UI themes.
type ThemeSelector struct {
	themes      []ThemeInfo
	selectedIdx int
	filter      string
	focused     bool
	onSelect    func(theme ThemeInfo)
	onCancel    func()
}

// NewThemeSelector creates a theme selector dialog.
func NewThemeSelector(themes []ThemeInfo) *ThemeSelector {
	return &ThemeSelector{themes: themes}
}

func (ts *ThemeSelector) SetOnSelect(fn func(theme ThemeInfo)) {
	ts.onSelect = fn
}

func (ts *ThemeSelector) SetOnCancel(fn func()) {
	ts.onCancel = fn
}

func (ts *ThemeSelector) SetFocused(focused bool) {
	ts.focused = focused
}

func (ts *ThemeSelector) Focused() bool {
	return ts.focused
}

func (ts *ThemeSelector) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	reset := "\x1b[0m"
	bold := "\x1b[1m"
	dim := "\x1b[2m"
	cyan := "\x1b[36m"
	green := "\x1b[32m"

	var lines []string
	lines = append(lines, bold+cyan+"Theme Selection"+reset)
	lines = append(lines, "")

	for i, theme := range ts.themes {
		prefix := "  "
		if i == ts.selectedIdx && ts.focused {
			prefix = "\u203a "
		}

		current := ""
		if theme.IsCurrent {
			current = green + " \u25CF" + reset
		}

		builtin := ""
		if theme.IsBuiltin {
			builtin = dim + " (built-in)" + reset
		}

		line := fmt.Sprintf("%s%s%s%s%s", prefix, bold+theme.Name+reset, current, builtin, reset)
		if tui.VisibleWidth(line) > width {
			line = tui.SliceByColumn(line, 0, width, true)
		}
		lines = append(lines, line)
	}

	if len(ts.themes) == 0 {
		lines = append(lines, dim+"  No themes available"+reset)
	}

	return lines
}

func (ts *ThemeSelector) HandleInput(data string) {
	switch {
	case tui.MatchesKey(data, "up") || tui.MatchesKey(data, "ctrl+p"):
		if ts.selectedIdx > 0 {
			ts.selectedIdx--
		}
	case tui.MatchesKey(data, "down") || tui.MatchesKey(data, "ctrl+n"):
		if ts.selectedIdx < len(ts.themes)-1 {
			ts.selectedIdx++
		}
	case tui.MatchesKey(data, "enter"):
		if ts.selectedIdx < len(ts.themes) && ts.onSelect != nil {
			ts.onSelect(ts.themes[ts.selectedIdx])
		}
	case tui.MatchesKey(data, "escape"):
		if ts.onCancel != nil {
			ts.onCancel()
		}
	}
}

func (ts *ThemeSelector) Invalidate()           {}
func (ts *ThemeSelector) WantsKeyRelease() bool { return false }

// ConfigSelector selects configuration profiles.
type ConfigSelector struct {
	profiles    []string
	selectedIdx int
	focused     bool
	onSelect    func(profile string)
	onCancel    func()
}

// NewConfigSelector creates a config profile selector.
func NewConfigSelector(profiles []string) *ConfigSelector {
	return &ConfigSelector{profiles: profiles}
}

func (cs *ConfigSelector) SetOnSelect(fn func(profile string)) {
	cs.onSelect = fn
}

func (cs *ConfigSelector) SetOnCancel(fn func()) {
	cs.onCancel = fn
}

func (cs *ConfigSelector) SetFocused(focused bool) {
	cs.focused = focused
}

func (cs *ConfigSelector) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	reset := "\x1b[0m"
	bold := "\x1b[1m"
	dim := "\x1b[2m"
	cyan := "\x1b[36m"

	var lines []string
	lines = append(lines, bold+cyan+"Configuration Profile"+reset)
	lines = append(lines, "")

	for i, profile := range cs.profiles {
		prefix := "  "
		if i == cs.selectedIdx && cs.focused {
			prefix = "\u203a " + cyan
		}
		lines = append(lines, fmt.Sprintf("%s%s", prefix, profile))
	}

	if len(cs.profiles) == 0 {
		lines = append(lines, dim+"  No profiles"+reset)
	}

	return lines
}

func (cs *ConfigSelector) HandleInput(data string) {
	switch {
	case tui.MatchesKey(data, "up") || tui.MatchesKey(data, "ctrl+p"):
		if cs.selectedIdx > 0 {
			cs.selectedIdx--
		}
	case tui.MatchesKey(data, "down") || tui.MatchesKey(data, "ctrl+n"):
		if cs.selectedIdx < len(cs.profiles)-1 {
			cs.selectedIdx++
		}
	case tui.MatchesKey(data, "enter"):
		if cs.selectedIdx < len(cs.profiles) && cs.onSelect != nil {
			cs.onSelect(cs.profiles[cs.selectedIdx])
		}
	case tui.MatchesKey(data, "escape"):
		if cs.onCancel != nil {
			cs.onCancel()
		}
	}
}

func (cs *ConfigSelector) Invalidate()           {}
func (cs *ConfigSelector) WantsKeyRelease() bool { return false }

// ScopedModelsSelector selects models for a specific scope.
type ScopedModelsSelector struct {
	models      []string
	selectedIdx int
	scope       string
	focused     bool
	onSelect    func(model string)
	onCancel    func()
}

// NewScopedModelsSelector creates a scoped model selector.
func NewScopedModelsSelector(scope string, models []string) *ScopedModelsSelector {
	return &ScopedModelsSelector{
		models: models,
		scope:  scope,
	}
}

func (sms *ScopedModelsSelector) SetOnSelect(fn func(model string)) {
	sms.onSelect = fn
}

func (sms *ScopedModelsSelector) SetOnCancel(fn func()) {
	sms.onCancel = fn
}

func (sms *ScopedModelsSelector) SetFocused(focused bool) {
	sms.focused = focused
}

func (sms *ScopedModelsSelector) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	reset := "\x1b[0m"
	bold := "\x1b[1m"
_ = "\x1b[2m"
	cyan := "\x1b[36m"

	var lines []string
	lines = append(lines, bold+cyan+"Scope: "+sms.scope+reset)
	lines = append(lines, "")

	for i, m := range sms.models {
		prefix := "  "
		if i == sms.selectedIdx && sms.focused {
			prefix = "\u203a " + cyan
		}
		lines = append(lines, fmt.Sprintf("%s%s", prefix, m))
	}

	return lines
}

func (sms *ScopedModelsSelector) HandleInput(data string) {
	switch {
	case tui.MatchesKey(data, "up") || tui.MatchesKey(data, "ctrl+p"):
		if sms.selectedIdx > 0 {
			sms.selectedIdx--
		}
	case tui.MatchesKey(data, "down") || tui.MatchesKey(data, "ctrl+n"):
		if sms.selectedIdx < len(sms.models)-1 {
			sms.selectedIdx++
		}
	case tui.MatchesKey(data, "enter"):
		if sms.selectedIdx < len(sms.models) && sms.onSelect != nil {
			sms.onSelect(sms.models[sms.selectedIdx])
		}
	case tui.MatchesKey(data, "escape"):
		if sms.onCancel != nil {
			sms.onCancel()
		}
	}
}

func (sms *ScopedModelsSelector) Invalidate()           {}
func (sms *ScopedModelsSelector) WantsKeyRelease() bool { return false }
