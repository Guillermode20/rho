package tui

import (
	"fmt"
	"strconv"
	"strings"
)

// SettingItem represents a single setting in the settings list.
type SettingItem struct {
	Key          string      `json:"key"`
	Label        string      `json:"label"`
	Description  string      `json:"description,omitempty"`
	Value        interface{} `json:"value"`
	Type         string      `json:"type"` // "boolean", "string", "number", "select"
	Options      []string    `json:"options,omitempty"` // for "select" type
	Placeholder  string      `json:"placeholder,omitempty"`
}

// SettingsListTheme defines styling for settings.
type SettingsListTheme struct {
	Label    func(text string) string
	Value    func(text string) string
	Desc     func(text string) string
	Selected func(text string) string
}

// DefaultSettingsListTheme returns a default theme.
func DefaultSettingsListTheme() SettingsListTheme {
	dim := "\x1b[2m"
	reset := "\x1b[0m"
	bold := "\x1b[1m"
	cyan := "\x1b[36m"

	return SettingsListTheme{
		Label:    func(text string) string { return bold + text + reset },
		Value:    func(text string) string { return cyan + text + reset },
		Desc:     func(text string) string { return dim + text + reset },
		Selected: func(text string) string { return "▸ " + bold + text + reset },
	}
}

// SettingsList is a scrollable settings editor component.
type SettingsList struct {
	items         []SettingItem
	selectedIndex int
	maxVisible    int
	theme         SettingsListTheme
	onChange      func(key string, value interface{})
	focused       bool
}

// NewSettingsList creates a new SettingsList.
func NewSettingsList(items []SettingItem, maxVisible int, theme SettingsListTheme) *SettingsList {
	return &SettingsList{
		items:      items,
		maxVisible: maxVisible,
		theme:      theme,
	}
}

// SetOnChange sets the change callback.
func (sl *SettingsList) SetOnChange(fn func(key string, value interface{})) {
	sl.onChange = fn
}

func (sl *SettingsList) SetFocused(focused bool) {
	sl.focused = focused
}

func (sl *SettingsList) Focused() bool {
	return sl.focused
}

func (sl *SettingsList) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	var lines []string

	// Calculate visible range
	totalItems := len(sl.items)
	if totalItems == 0 {
		lines = append(lines, "  No settings")
		return lines
	}

	startIndex := max(0, min(sl.selectedIndex-sl.maxVisible/2, totalItems-sl.maxVisible))
	endIndex := min(startIndex+sl.maxVisible, totalItems)

	labelWidth := 0
	for i := startIndex; i < endIndex; i++ {
		w := VisibleWidth(sl.items[i].Label)
		if w > labelWidth {
			labelWidth = w
		}
	}
	labelWidth = min(labelWidth, width/2)

	for i := startIndex; i < endIndex; i++ {
		item := sl.items[i]
		isSelected := i == sl.selectedIndex

		label := item.Label
		if VisibleWidth(label) > labelWidth {
			label = SliceByColumn(label, 0, labelWidth, true)
		}

		valueStr := formatValue(item)
		valueW := VisibleWidth(valueStr)
		maxValueW := width - labelWidth - 4
		if valueW > maxValueW {
			valueStr = SliceByColumn(valueStr, 0, maxValueW, true)
		}

		padding := labelWidth - VisibleWidth(label) + 2
		line := ""
		if isSelected && sl.focused {
			line = sl.theme.Selected(label) + strings.Repeat(" ", padding) + sl.theme.Value(valueStr)
		} else {
			line = "  " + sl.theme.Label(label) + strings.Repeat(" ", padding) + sl.theme.Value(valueStr)
		}

		// Ensure line fills width
		lineW := VisibleWidth(line)
		if lineW < width {
			line += strings.Repeat(" ", width-lineW)
		}

		lines = append(lines, line)

		// Description
		if isSelected && item.Description != "" {
			desc := sl.theme.Desc("  " + item.Description)
			descW := VisibleWidth(desc)
			if descW > width {
				desc = SliceByColumn(desc, 0, width, true)
			} else if descW < width {
				desc += strings.Repeat(" ", width-descW)
			}
			lines = append(lines, desc)
		}
	}

	// Scroll indicator
	if endIndex < totalItems {
		scrollInfo := "..."
		lines = append(lines, scrollInfo+strings.Repeat(" ", max(0, width-VisibleWidth(scrollInfo))))
	}

	return lines
}

func formatValue(item SettingItem) string {
	switch item.Type {
	case "boolean":
		if b, ok := item.Value.(bool); ok && b {
			return "✓ on"
		}
		return "✗ off"
	case "number":
		switch v := item.Value.(type) {
		case float64:
			return fmt.Sprintf("%.1f", v)
		case int:
			return strconv.Itoa(v)
		default:
			return fmt.Sprintf("%v", v)
		}
	case "select":
		if item.Value != nil {
			return fmt.Sprintf("%v", item.Value)
		}
		return "(select)"
	default:
		if item.Value != nil {
			return fmt.Sprintf("%v", item.Value)
		}
		return item.Placeholder
	}
}

func (sl *SettingsList) HandleInput(data string) {
	if !sl.focused {
		return
	}

	switch {
	case MatchesKey(data, "up") || MatchesKey(data, "ctrl+p"):
		if sl.selectedIndex > 0 {
			sl.selectedIndex--
		}
	case MatchesKey(data, "down") || MatchesKey(data, "ctrl+n"):
		if sl.selectedIndex < len(sl.items)-1 {
			sl.selectedIndex++
		}
	case MatchesKey(data, "left"):
		sl.toggleValue(-1)
	case MatchesKey(data, "right"):
		sl.toggleValue(1)
	case MatchesKey(data, "enter"):
		sl.toggleValue(0)
	}
}

func (sl *SettingsList) toggleValue(direction int) {
	if sl.selectedIndex >= len(sl.items) {
		return
	}
	item := &sl.items[sl.selectedIndex]

	switch item.Type {
	case "boolean":
		if b, ok := item.Value.(bool); ok {
			item.Value = !b
			if sl.onChange != nil {
				sl.onChange(item.Key, item.Value)
			}
		} else {
			item.Value = true
			if sl.onChange != nil {
				sl.onChange(item.Key, item.Value)
			}
		}
	case "select":
		if len(item.Options) > 0 {
			currentIdx := 0
			currentStr := fmt.Sprintf("%v", item.Value)
			for i, opt := range item.Options {
				if opt == currentStr {
					currentIdx = i
					break
				}
			}
			newIdx := currentIdx + direction
			if direction == 0 {
				newIdx = (currentIdx + 1) % len(item.Options)
			}
			if newIdx < 0 {
				newIdx = len(item.Options) - 1
			} else if newIdx >= len(item.Options) {
				newIdx = 0
			}
			item.Value = item.Options[newIdx]
			if sl.onChange != nil {
				sl.onChange(item.Key, item.Value)
			}
		}
	case "number":
		if direction != 0 {
			switch v := item.Value.(type) {
			case float64:
				item.Value = v + float64(direction)
				if sl.onChange != nil {
					sl.onChange(item.Key, item.Value)
				}
			case int:
				item.Value = v + direction
				if sl.onChange != nil {
					sl.onChange(item.Key, item.Value)
				}
			}
		}
	}
}

func (sl *SettingsList) Invalidate() {}
func (sl *SettingsList) WantsKeyRelease() bool { return false }
