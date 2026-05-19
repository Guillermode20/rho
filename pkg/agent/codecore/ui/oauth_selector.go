package codecui

import (
	"fmt"
	"strings"

	"github.com/earendil-works/rho/pkg/tui"
)

// OAuthProviderInfo describes an OAuth provider for the selector.
type OAuthProviderInfo struct {
	ID          string
	Name        string
	Description string
	IsLoggedIn  bool
	BaseURL     string
}

// OAuthSelectorComponent renders an OAuth provider selection dialog.
type OAuthSelectorComponent struct {
	providers     []OAuthProviderInfo
	selectedIndex int
	maxVisible    int
	onSelect      func(provider OAuthProviderInfo)
	onCancel      func()
	focused       bool
}

// NewOAuthSelectorComponent creates a new OAuth selector.
func NewOAuthSelectorComponent(providers []OAuthProviderInfo, maxVisible int) *OAuthSelectorComponent {
	return &OAuthSelectorComponent{
		providers:  providers,
		maxVisible: maxVisible,
	}
}

// SetOnSelect sets the select callback.
func (s *OAuthSelectorComponent) SetOnSelect(fn func(provider OAuthProviderInfo)) {
	s.onSelect = fn
}

// SetOnCancel sets the cancel callback.
func (s *OAuthSelectorComponent) SetOnCancel(fn func()) {
	s.onCancel = fn
}

func (s *OAuthSelectorComponent) SetFocused(focused bool) { s.focused = focused }
func (s *OAuthSelectorComponent) Focused() bool            { return s.focused }

// UpdateProviders refreshes the provider list.
func (s *OAuthSelectorComponent) UpdateProviders(providers []OAuthProviderInfo) {
	s.providers = providers
	if s.selectedIndex >= len(s.providers) {
		s.selectedIndex = len(s.providers) - 1
	}
	if s.selectedIndex < 0 {
		s.selectedIndex = 0
	}
}

func (s *OAuthSelectorComponent) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	dialogWidth := 60
	if width < dialogWidth {
		dialogWidth = width - 4
	}
	if dialogWidth < 40 {
		dialogWidth = 40
	}

	var lines []string

	// Header
	lines = append(lines, "┌"+strings.Repeat("─", dialogWidth-2)+"┐")
	title := "  Choose an OAuth Provider"
	lines = append(lines, fmt.Sprintf("│%s%s│", title, strings.Repeat(" ", dialogWidth-2-len(title))))
	lines = append(lines, "│"+strings.Repeat("─", dialogWidth-2)+"│")

	if len(s.providers) == 0 {
		msg := "  No OAuth providers configured"
		lines = append(lines, fmt.Sprintf("│%s%s│", msg, strings.Repeat(" ", dialogWidth-2-len(msg))))
		lines = append(lines, "└"+strings.Repeat("─", dialogWidth-2)+"┘")
		return lines
	}

	// Visible range
	total := len(s.providers)
	startIdx := 0
	if s.selectedIndex > s.maxVisible/2 {
		startIdx = s.selectedIndex - s.maxVisible/2
	}
	if startIdx+s.maxVisible > total {
		startIdx = total - s.maxVisible
	}
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + s.maxVisible
	if endIdx > total {
		endIdx = total
	}

	for i := startIdx; i < endIdx; i++ {
		p := s.providers[i]
		prefix := "  "
		if i == s.selectedIndex {
			prefix = "▸ "
		}

		statusIcon := " \x1b[90m○\x1b[0m"
		statusColor := "\x1b[90m" // dim gray
		if p.IsLoggedIn {
			statusIcon = " \x1b[32m●\x1b[0m"
			statusColor = "\x1b[32m" // green
		}

		name := fmt.Sprintf("%s%s%s%s", prefix, statusColor, p.Name, "\x1b[0m")
		desc := p.Description
		if desc != "" {
			desc = " \x1b[2m" + desc + "\x1b[0m"
		}

		line := fmt.Sprintf("│%s%s%s%s│",
			name,
			desc,
			strings.Repeat(" ", max(0, dialogWidth-2-len(stripAnsi(name))-len(stripAnsi(desc))-1)),
			statusIcon,
		)
		lines = append(lines, line)
	}

	// Scroll indicator
	if endIdx < total {
		more := fmt.Sprintf("  ... %d more providers", total-endIdx)
		lines = append(lines, fmt.Sprintf("│%s%s│", more, strings.Repeat(" ", dialogWidth-2-len(more))))
	}

	// Hint
	hint := "  ↑↓ navigate · Enter select · Esc cancel"
	lines = append(lines, "│"+strings.Repeat("─", dialogWidth-2)+"│")
	lines = append(lines, fmt.Sprintf("│%s%s│", hint, strings.Repeat(" ", dialogWidth-2-len(hint))))
	lines = append(lines, "└"+strings.Repeat("─", dialogWidth-2)+"┘")

	return lines
}

func (s *OAuthSelectorComponent) HandleInput(data string) {
	switch {
	case tui.MatchesKey(data, "up") || tui.MatchesKey(data, "ctrl+p"):
		if s.selectedIndex > 0 {
			s.selectedIndex--
		}
	case tui.MatchesKey(data, "down") || tui.MatchesKey(data, "ctrl+n"):
		if s.selectedIndex < len(s.providers)-1 {
			s.selectedIndex++
		}
	case tui.MatchesKey(data, "enter"):
		if s.selectedIndex < len(s.providers) && s.onSelect != nil {
			s.onSelect(s.providers[s.selectedIndex])
		}
	case tui.MatchesKey(data, "escape"):
		if s.onCancel != nil {
			s.onCancel()
		}
	}
}

func (s *OAuthSelectorComponent) Invalidate()          {}
func (s *OAuthSelectorComponent) WantsKeyRelease() bool { return false }

func stripAnsi(s string) string {
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '~' {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
