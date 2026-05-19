package codecui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/earendil-works/rho/pkg/tui"
)

// SettingType defines the type of a setting.
type SettingType string

const (
	SettingBool   SettingType = "boolean"
	SettingString SettingType = "string"
	SettingNumber SettingType = "number"
	SettingSelect SettingType = "select"
)

// SettingDef defines a single setting.
type SettingDef struct {
	Key         string
	Label       string
	Description string
	Type        SettingType
	Value       interface{}
	Default     interface{}
	Options     []string // for select type
	Min         float64  // for number type
	Max         float64
	Category    string
}

// SettingsSelector is a dialog for browsing and editing settings.
type SettingsSelector struct {
	settings    []SettingDef
	categories  []string
	catSettings map[string][]SettingDef
	selectedCat int
	selectedIdx int
	focused     bool
	filter      string
	onChange    func(key string, value interface{})
	onCancel    func()
}

// NewSettingsSelector creates a settings editor dialog.
func NewSettingsSelector(settings []SettingDef) *SettingsSelector {
	cats := make(map[string][]SettingDef)
	var catOrder []string
	for _, s := range settings {
		cat := s.Category
		if cat == "" {
			cat = "General"
		}
		if _, ok := cats[cat]; !ok {
			catOrder = append(catOrder, cat)
		}
		cats[cat] = append(cats[cat], s)
	}
	return &SettingsSelector{
		settings:    settings,
		categories:  catOrder,
		catSettings: cats,
	}
}

// SetOnChange sets the change callback.
func (ss *SettingsSelector) SetOnChange(fn func(key string, value interface{})) {
	ss.onChange = fn
}

// SetOnCancel sets the cancel callback.
func (ss *SettingsSelector) SetOnCancel(fn func()) {
	ss.onCancel = fn
}

func (ss *SettingsSelector) SetFocused(focused bool) {
	ss.focused = focused
}

func (ss *SettingsSelector) Focused() bool {
	return ss.focused
}

func (ss *SettingsSelector) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	reset := "\x1b[0m"
	bold := "\x1b[1m"
	dim := "\x1b[2m"
	cyan := "\x1b[36m"
_ = "\x1b[33m"
	green := "\x1b[32m"

	var lines []string
	lines = append(lines, bold+cyan+"Settings"+reset)
	lines = append(lines, dim+"  Search: "+ss.filter+"_"+reset)
	lines = append(lines, "")

	// Get filtered settings
	filtered := ss.getFilteredSettings()
	if len(filtered) == 0 {
		lines = append(lines, dim+"  No settings"+reset)
		return lines
	}

	for i, s := range filtered {
		prefix := "  "
		if i == ss.selectedIdx && ss.focused {
			prefix = "\u203a " + cyan
		}

		valueStr := formatSettingValue(s)
		valueColor := dim
		if s.Type == SettingBool {
			if b, ok := s.Value.(bool); ok && b {
				valueColor = green
			}
		}

		line := fmt.Sprintf("%s%s  %s%s%s", prefix,
			bold+s.Label+reset,
			dim+" ("+string(s.Type)+")"+reset, valueColor+valueStr+reset, reset)

		if tui.VisibleWidth(line) > width {
			line = tui.SliceByColumn(line, 0, width, true)
		}
		lines = append(lines, line)

		// Show description for selected item
		if i == ss.selectedIdx && s.Description != "" {
			lines = append(lines, dim+"  "+s.Description+reset)
		}
	}

	return lines
}

func (ss *SettingsSelector) getFilteredSettings() []SettingDef {
	if ss.filter == "" {
		return ss.settings
	}
	lower := strings.ToLower(ss.filter)
	var filtered []SettingDef
	for _, s := range ss.settings {
		if strings.Contains(strings.ToLower(s.Label), lower) ||
			strings.Contains(strings.ToLower(s.Key), lower) ||
			strings.Contains(strings.ToLower(s.Description), lower) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

func formatSettingValue(s SettingDef) string {
	switch s.Type {
	case SettingBool:
		if b, ok := s.Value.(bool); ok && b {
			return "on"
		}
		return "off"
	case SettingString:
		if s, ok := s.Value.(string); ok && s != "" {
			return s
		}
		return "(empty)"
	case SettingNumber:
		switch v := s.Value.(type) {
		case float64:
			return strconv.FormatFloat(v, 'f', 1, 64)
		case int:
			return strconv.Itoa(v)
		default:
			return fmt.Sprintf("%v", v)
		}
	case SettingSelect:
		if s, ok := s.Value.(string); ok {
			return s
		}
		return "(select)"
	}
	return fmt.Sprintf("%v", s.Value)
}

func (ss *SettingsSelector) HandleInput(data string) {
	switch {
	case tui.MatchesKey(data, "up") || tui.MatchesKey(data, "ctrl+p"):
		if ss.selectedIdx > 0 {
			ss.selectedIdx--
		}
	case tui.MatchesKey(data, "down") || tui.MatchesKey(data, "ctrl+n"):
		filtered := ss.getFilteredSettings()
		if ss.selectedIdx < len(filtered)-1 {
			ss.selectedIdx++
		}
	case tui.MatchesKey(data, "left"):
		ss.toggleValue(-1)
	case tui.MatchesKey(data, "right"):
		ss.toggleValue(1)
	case tui.MatchesKey(data, "enter"):
		ss.toggleValue(0)
	case tui.MatchesKey(data, "escape"):
		if ss.onCancel != nil {
			ss.onCancel()
		}
	case tui.MatchesKey(data, "backspace"):
		if len(ss.filter) > 0 {
			ss.filter = ss.filter[:len(ss.filter)-1]
			ss.selectedIdx = 0
		}
	default:
		if len(data) == 1 && data[0] >= 0x20 && data[0] <= 0x7e {
			ss.filter += string(data[0])
			ss.selectedIdx = 0
		}
	}
}

func (ss *SettingsSelector) toggleValue(direction int) {
	filtered := ss.getFilteredSettings()
	if ss.selectedIdx >= len(filtered) {
		return
	}
	s := &filtered[ss.selectedIdx]

	switch s.Type {
	case SettingBool:
		if b, ok := s.Value.(bool); ok {
			s.Value = !b
		} else {
			s.Value = true
		}
		if ss.onChange != nil {
			ss.onChange(s.Key, s.Value)
		}

	case SettingSelect:
		if len(s.Options) > 0 {
			current := fmt.Sprintf("%v", s.Value)
			idx := 0
			for i, o := range s.Options {
				if o == current {
					idx = i
					break
				}
			}
			if direction == 0 {
				idx = (idx + 1) % len(s.Options)
			} else {
				idx += direction
				if idx < 0 {
					idx = len(s.Options) - 1
				} else if idx >= len(s.Options) {
					idx = 0
				}
			}
			s.Value = s.Options[idx]
			if ss.onChange != nil {
				ss.onChange(s.Key, s.Value)
			}
		}

	case SettingNumber:
		if direction != 0 {
			switch v := s.Value.(type) {
			case float64:
				s.Value = v + float64(direction)
				if ss.onChange != nil {
					ss.onChange(s.Key, s.Value)
				}
			case int:
				s.Value = v + direction
				if ss.onChange != nil {
					ss.onChange(s.Key, s.Value)
				}
			}
		}
	}
}

func (ss *SettingsSelector) Invalidate()           {}
func (ss *SettingsSelector) WantsKeyRelease() bool { return false }
