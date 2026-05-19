package codecui

import (
	"fmt"
	"strings"
	"time"
)

// Announcement represents a single release announcement.
type Announcement struct {
	Version   string `json:"version"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Date      string `json:"date"`
	IsNew     bool   `json:"isNew"`
}

// EarendilAnnouncement displays release announcements and tips at startup.
type EarendilAnnouncement struct {
	announcements []Announcement
	dismissed     bool
	showTips      bool
}

// NewEarendilAnnouncement creates a new announcement component.
func NewEarendilAnnouncement() *EarendilAnnouncement {
	return &EarendilAnnouncement{
		announcements: defaultAnnouncements(),
		showTips:      true,
	}
}

// Dismiss hides all announcements.
func (e *EarendilAnnouncement) Dismiss() {
	e.dismissed = true
}

// SetShowTips controls whether tips are shown.
func (e *EarendilAnnouncement) SetShowTips(show bool) {
	e.showTips = show
}

// Render returns the announcement display lines.
func (e *EarendilAnnouncement) Render(width int) []string {
	if e.dismissed || width <= 0 {
		return nil
	}

	var lines []string

	// Header
	lines = append(lines, "")
	lines = append(lines, styleBold("✦ rho — your coding companion ✦"))
	lines = append(lines, "")

	// Version info
	lines = append(lines, styleDim(fmt.Sprintf("Version %s — %s", "0.2.0", time.Now().Format("January 2006"))))
	lines = append(lines, "")

	// New announcements
	for _, a := range e.announcements {
		if !a.IsNew {
			continue
		}
		lines = append(lines, styleBold(fmt.Sprintf("▼ %s — %s", a.Version, a.Title)))
		for _, bodyLine := range strings.Split(a.Body, "\n") {
			if strings.TrimSpace(bodyLine) == "" {
				continue
			}
			lines = append(lines, "  "+bodyLine)
		}
		lines = append(lines, "")
	}

	// Tips
	if e.showTips {
		lines = append(lines, styleDim("Tips:"))
		tips := []string{
			"• Type your message and press Enter to start",
			"• Use /help to see available commands",
			"• Ctrl+C to quit, Ctrl+D for debug info",
			"• Extensions go in ~/.rho/extensions/",
		}
		for _, tip := range tips {
			lines = append(lines, styleDim("  "+tip))
		}
		lines = append(lines, "")
	}

	return lines
}

func styleBold(s string) string { return "\x1b[1m" + s + "\x1b[0m" }
func styleDim(s string) string  { return "\x1b[2m" + s + "\x1b[0m" }
func styleCyan(s string) string { return "\x1b[36m" + s + "\x1b[0m" }

func defaultAnnouncements() []Announcement {
	return []Announcement{
		{
			Version: "0.2.0",
			Title:   "Full pi parity",
			Date:    "2025-05-19",
			IsNew:   true,
			Body:    "Complete Go translation of pi: all AI providers, agent loop,\nextension system with 12 lifecycle hooks, TUI with differential\nrendering, session management, compaction, themes, skills, and more.",
		},
	}
}

var _ = styleCyan
