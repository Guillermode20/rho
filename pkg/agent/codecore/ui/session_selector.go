package codecui

import (
	"fmt"
	"strings"
	"time"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/tui"
)

// SessionInfo is session metadata for display.
type SessionInfo struct {
	ID           string
	Timestamp    time.Time
	CWD          string
	MessageCount int
	Preview      string
	IsActive     bool
}

// SessionSelector is a dialog for browsing and selecting sessions.
type SessionSelector struct {
	sessions    []SessionInfo
	selectedIdx int
	filter      string
	focused     bool
	onSelect    func(session SessionInfo)
	onCancel    func()
}

// NewSessionSelector creates a session selector dialog.
func NewSessionSelector(sessions []SessionInfo) *SessionSelector {
	return &SessionSelector{
		sessions: sessions,
	}
}

// SetOnSelect sets the selection callback.
func (ss *SessionSelector) SetOnSelect(fn func(session SessionInfo)) {
	ss.onSelect = fn
}

// SetOnCancel sets the cancel callback.
func (ss *SessionSelector) SetOnCancel(fn func()) {
	ss.onCancel = fn
}

func (ss *SessionSelector) SetFocused(focused bool) {
	ss.focused = focused
}

func (ss *SessionSelector) Focused() bool {
	return ss.focused
}

// getFilteredSessions returns sessions matching the filter.
func (ss *SessionSelector) getFilteredSessions() []SessionInfo {
	if ss.filter == "" {
		return ss.sessions
	}
	lower := strings.ToLower(ss.filter)
	var filtered []SessionInfo
	for _, s := range ss.sessions {
		if strings.Contains(strings.ToLower(s.ID), lower) ||
			strings.Contains(strings.ToLower(s.Preview), lower) ||
			strings.Contains(strings.ToLower(s.CWD), lower) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

func (ss *SessionSelector) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	reset := "\x1b[0m"
	bold := "\x1b[1m"
	dim := "\x1b[2m"
	cyan := "\x1b[36m"
	green := "\x1b[32m"
_ = "\x1b[33m"

	var lines []string
	lines = append(lines, bold+cyan+"Session Manager"+reset)
	lines = append(lines, dim+"  Search: "+ss.filter+"_"+reset)
	lines = append(lines, "")

	filtered := ss.getFilteredSessions()
	if len(filtered) == 0 {
		lines = append(lines, dim+"  No sessions found"+reset)
		return lines
	}

	maxVisible := 10
	startIdx := 0
	if ss.selectedIdx > maxVisible/2 {
		startIdx = ss.selectedIdx - maxVisible/2
	}
	if startIdx+maxVisible > len(filtered) {
		startIdx = len(filtered) - maxVisible
	}
	if startIdx < 0 {
		startIdx = 0
	}

	for i := startIdx; i < len(filtered) && i < startIdx+maxVisible; i++ {
		s := filtered[i]
		isSelected := i == ss.selectedIdx && ss.focused

		prefix := "  "
		if isSelected {
			prefix = "\u203a " + cyan
		}

		active := ""
		if s.IsActive {
			active = green + " \u25CF" + reset
		}

		timeStr := s.Timestamp.Format("Jan 02 15:04")
		line := fmt.Sprintf("%s%s%s%s  %s  %s", prefix, bold+timeStr+reset, active,
			dim+fmt.Sprintf(" (%d msgs)", s.MessageCount)+reset, s.Preview, reset)

		if tui.VisibleWidth(line) > width {
			line = tui.SliceByColumn(line, 0, width, true)
		}
		lines = append(lines, line)
	}

	if len(filtered) > maxVisible {
		lines = append(lines, dim+fmt.Sprintf("  ... %d more sessions", len(filtered)-maxVisible)+reset)
	}

	return lines
}

func (ss *SessionSelector) HandleInput(data string) {
	switch {
	case tui.MatchesKey(data, "up") || tui.MatchesKey(data, "ctrl+p"):
		if ss.selectedIdx > 0 {
			ss.selectedIdx--
		}
	case tui.MatchesKey(data, "down") || tui.MatchesKey(data, "ctrl+n"):
		filtered := ss.getFilteredSessions()
		if ss.selectedIdx < len(filtered)-1 {
			ss.selectedIdx++
		}
	case tui.MatchesKey(data, "enter"):
		filtered := ss.getFilteredSessions()
		if ss.selectedIdx < len(filtered) && ss.onSelect != nil {
			ss.onSelect(filtered[ss.selectedIdx])
		}
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

func (ss *SessionSelector) Invalidate()           {}
func (ss *SessionSelector) WantsKeyRelease() bool { return false }

// SessionSelectorSearch is a specialized session search with full-text.
type SessionSelectorSearch struct {
	*SessionSelector
}

// NewSessionSelectorSearch creates a searchable session selector.
func NewSessionSelectorSearch(sessions []SessionInfo) *SessionSelectorSearch {
	return &SessionSelectorSearch{
		SessionSelector: NewSessionSelector(sessions),
	}
}

// TreeSelector navigates session tree branches.
type TreeSelector struct {
	entries     []SessionTreeEntry
	selectedIdx int
	focused     bool
	onSelect    func(entry SessionTreeEntry)
	onCancel    func()
}

// SessionTreeEntry represents a node in the session tree.
type SessionTreeEntry struct {
	ID        string
	Label     string
	Depth     int
	IsBranch  bool
	IsCurrent bool
}

// NewTreeSelector creates a tree navigation selector.
func NewTreeSelector(entries []SessionTreeEntry) *TreeSelector {
	return &TreeSelector{entries: entries}
}

func (ts *TreeSelector) SetOnSelect(fn func(entry SessionTreeEntry)) {
	ts.onSelect = fn
}

func (ts *TreeSelector) SetOnCancel(fn func()) {
	ts.onCancel = fn
}

func (ts *TreeSelector) SetFocused(focused bool) {
	ts.focused = focused
}

func (ts *TreeSelector) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	reset := "\x1b[0m"
	bold := "\x1b[1m"
_ = "\x1b[2m"
	cyan := "\x1b[36m"
	green := "\x1b[32m"

	var lines []string
	lines = append(lines, bold+cyan+"Session Tree"+reset)
	lines = append(lines, "")

	for i, entry := range ts.entries {
		prefix := "  "
		if i == ts.selectedIdx && ts.focused {
			prefix = "\u203a "
		}

		indent := strings.Repeat("  ", entry.Depth)
		branchChar := "\u2502"
		if entry.IsBranch {
			branchChar = "\u2514"
		}

		label := entry.Label
		if entry.IsCurrent {
			label = green + label + reset
		}

		line := fmt.Sprintf("%s%s%s %s %s", prefix, indent, branchChar, label, reset)
		lines = append(lines, line)
	}

	return lines
}

func (ts *TreeSelector) HandleInput(data string) {
	switch {
	case tui.MatchesKey(data, "up") || tui.MatchesKey(data, "ctrl+p"):
		if ts.selectedIdx > 0 {
			ts.selectedIdx--
		}
	case tui.MatchesKey(data, "down") || tui.MatchesKey(data, "ctrl+n"):
		if ts.selectedIdx < len(ts.entries)-1 {
			ts.selectedIdx++
		}
	case tui.MatchesKey(data, "enter"):
		if ts.selectedIdx < len(ts.entries) && ts.onSelect != nil {
			ts.onSelect(ts.entries[ts.selectedIdx])
		}
	case tui.MatchesKey(data, "escape"):
		if ts.onCancel != nil {
			ts.onCancel()
		}
	}
}

func (ts *TreeSelector) Invalidate()           {}
func (ts *TreeSelector) WantsKeyRelease() bool { return false }

// Ensure types used.
var _ = agent.SessionManager{}
