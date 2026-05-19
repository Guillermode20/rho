package tui

import (
	"strings"
)

// SelectItem represents an item in a select list.
type SelectItem struct {
	Value       string
	Label       string
	Description string
}

// SelectList is a scrollable selection list component.
type SelectList struct {
	items         []SelectItem
	filteredItems []SelectItem
	selectedIndex int
	maxVisible    int
	onSelect      func(item SelectItem)
	onCancel      func()
	focused       bool
}

// NewSelectList creates a new SelectList.
func NewSelectList(items []SelectItem, maxVisible int) *SelectList {
	filtered := make([]SelectItem, len(items))
	copy(filtered, items)
	return &SelectList{
		items:         items,
		filteredItems: filtered,
		maxVisible:    maxVisible,
	}
}

// SetOnSelect sets the callback for when an item is selected.
func (s *SelectList) SetOnSelect(fn func(item SelectItem)) {
	s.onSelect = fn
}

// SetOnCancel sets the callback for when selection is cancelled.
func (s *SelectList) SetOnCancel(fn func()) {
	s.onCancel = fn
}

// SetFilter filters items by the given string.
func (s *SelectList) SetFilter(filter string) {
	s.filteredItems = nil
	for _, item := range s.items {
		if strings.HasPrefix(strings.ToLower(item.Value), strings.ToLower(filter)) {
			s.filteredItems = append(s.filteredItems, item)
		}
	}
	if s.selectedIndex >= len(s.filteredItems) {
		s.selectedIndex = 0
	}
}

// SetSelectedIndex sets the selected index.
func (s *SelectList) SetSelectedIndex(index int) {
	if index < 0 {
		index = 0
	}
	if index >= len(s.filteredItems) {
		index = len(s.filteredItems) - 1
	}
	s.selectedIndex = index
}

func (s *SelectList) SetFocused(focused bool) {
	s.focused = focused
}

func (s *SelectList) Focused() bool {
	return s.focused
}

func (s *SelectList) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	var lines []string

	if len(s.filteredItems) == 0 {
		lines = append(lines, "  No matching items")
		return lines
	}

	// Calculate visible range with scrolling
	startIndex := max(0, min(s.selectedIndex-s.maxVisible/2, len(s.filteredItems)-s.maxVisible))
	endIndex := min(startIndex+s.maxVisible, len(s.filteredItems))

	// Primary column width
	primaryWidth := width - 4 // account for "> " prefix and padding
	if primaryWidth < 10 {
		primaryWidth = 10
	}

	for i := startIndex; i < endIndex; i++ {
		item := s.filteredItems[i]
		isSelected := i == s.selectedIndex

		prefix := "  "
		if isSelected {
			prefix = "> "
		}

		label := item.Label
		if VisibleWidth(label) > primaryWidth {
			label = SliceByColumn(label, 0, primaryWidth, true)
		}

		line := prefix + label

		if item.Description != "" && width > 40 {
			desc := strings.ReplaceAll(item.Description, "\n", " ")
			descMax := width - VisibleWidth(line) - 2
			if descMax > 10 {
				desc = SliceByColumn(desc, 0, descMax, true)
				// Pad to align description
				labelW := VisibleWidth(prefix + label)
				padding := width - labelW - VisibleWidth(desc) - 1
				if padding < 1 {
					padding = 1
				}
				line += strings.Repeat(" ", padding) + desc
			}
		}

		// Ensure line doesn't exceed width
		if VisibleWidth(line) > width {
			line = SliceByColumn(line, 0, width, true)
		} else if VisibleWidth(line) < width {
			line += strings.Repeat(" ", width-VisibleWidth(line))
		}

		lines = append(lines, line)
	}

	// Show scroll indicator if there are more items
	if endIndex < len(s.filteredItems) {
		scrollInfo := "..."
		lines = append(lines, scrollInfo+strings.Repeat(" ", max(0, width-VisibleWidth(scrollInfo))))
	}

	return lines
}

func (s *SelectList) HandleInput(data string) {
	switch {
	case MatchesKey(data, "enter"):
		if s.selectedIndex < len(s.filteredItems) && s.onSelect != nil {
			s.onSelect(s.filteredItems[s.selectedIndex])
		}
	case MatchesKey(data, "escape"):
		if s.onCancel != nil {
			s.onCancel()
		}
	case MatchesKey(data, "up") || MatchesKey(data, "ctrl+p"):
		if s.selectedIndex > 0 {
			s.selectedIndex--
		}
	case MatchesKey(data, "down") || MatchesKey(data, "ctrl+n"):
		if s.selectedIndex < len(s.filteredItems)-1 {
			s.selectedIndex++
		}
	case MatchesKey(data, "pageup"):
		s.selectedIndex -= s.maxVisible
		if s.selectedIndex < 0 {
			s.selectedIndex = 0
		}
	case MatchesKey(data, "pagedown"):
		s.selectedIndex += s.maxVisible
		if s.selectedIndex >= len(s.filteredItems) {
			s.selectedIndex = len(s.filteredItems) - 1
		}
	case MatchesKey(data, "home"):
		s.selectedIndex = 0
	case MatchesKey(data, "end"):
		s.selectedIndex = len(s.filteredItems) - 1
	}
}

func (s *SelectList) Invalidate() {}

func (s *SelectList) WantsKeyRelease() bool { return false }
