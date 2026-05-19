package codecui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/earendil-works/rho/pkg/tui"
)

// CountdownTimerComponent renders a countdown timer overlay for timed confirmations.
type CountdownTimerComponent struct {
	mu             sync.Mutex
	title          string
	message        string
	totalTime      time.Duration
	remaining      time.Duration
	startTime      time.Time
	autoDismiss    bool
	confirmed      bool
	dismissed      bool
	onConfirm      func()
	onDismiss      func()
	onTimeout      func()
}

// NewCountdownTimerComponent creates a new countdown timer.
func NewCountdownTimerComponent(title, message string, timeout time.Duration) *CountdownTimerComponent {
	return &CountdownTimerComponent{
		title:      title,
		message:    message,
		totalTime:  timeout,
		remaining:  timeout,
		startTime:  time.Now(),
		autoDismiss: true,
	}
}

// SetOnConfirm sets the confirm callback.
func (c *CountdownTimerComponent) SetOnConfirm(fn func()) {
	c.onConfirm = fn
}

// SetOnDismiss sets the dismiss callback.
func (c *CountdownTimerComponent) SetOnDismiss(fn func()) {
	c.onDismiss = fn
}

// SetOnTimeout sets the timeout callback.
func (c *CountdownTimerComponent) SetOnTimeout(fn func()) {
	c.onTimeout = fn
}

// Tick updates the remaining time. Returns true if timed out.
func (c *CountdownTimerComponent) Tick() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	elapsed := time.Since(c.startTime)
	c.remaining = c.totalTime - elapsed
	if c.remaining <= 0 {
		c.remaining = 0
		if c.autoDismiss && !c.dismissed && !c.confirmed {
			c.dismissed = true
			return true
		}
	}
	return false
}

// Confirm marks the timer as confirmed.
func (c *CountdownTimerComponent) Confirm() {
	c.mu.Lock()
	c.confirmed = true
	c.mu.Unlock()
	if c.onConfirm != nil {
		c.onConfirm()
	}
}

// Dismiss marks the timer as dismissed.
func (c *CountdownTimerComponent) Dismiss() {
	c.mu.Lock()
	c.dismissed = true
	c.mu.Unlock()
	if c.onDismiss != nil {
		c.onDismiss()
	}
}

func (c *CountdownTimerComponent) Render(width int) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if width <= 0 {
		return nil
	}

	dialogWidth := 50
	if width < dialogWidth {
		dialogWidth = width - 4
	}
	if dialogWidth < 30 {
		dialogWidth = 30
	}

	var lines []string

	// Borders
	lines = append(lines, "┌"+strings.Repeat("─", dialogWidth-2)+"┐")

	// Title
	paddedTitle := fmt.Sprintf("  %s", c.title)
	lines = append(lines, fmt.Sprintf("│%s%s│", paddedTitle, strings.Repeat(" ", dialogWidth-2-len(paddedTitle))))

	lines = append(lines, "│"+strings.Repeat("─", dialogWidth-2)+"│")

	if c.confirmed {
		lines = append(lines, fmt.Sprintf("│  \x1b[32m✓ Confirmed!\x1b[0m%s│", strings.Repeat(" ", dialogWidth-18)))
	} else if c.dismissed {
		lines = append(lines, fmt.Sprintf("│  \x1b[33m✗ Dismissed\x1b[0m%s│", strings.Repeat(" ", dialogWidth-16)))
	} else {
		// Message
		if c.message != "" {
			msg := c.message
			innerW := dialogWidth - 6
			if len(msg) > innerW {
				msg = msg[:innerW-3] + "..."
			}
			lines = append(lines, fmt.Sprintf("│  %s%s│", msg, strings.Repeat(" ", dialogWidth-4-len(msg))))
			lines = append(lines, "│" + strings.Repeat(" ", dialogWidth-2) + "│")
		}

		// Progress bar
		pct := 1.0
		if c.totalTime > 0 {
			pct = float64(c.remaining) / float64(c.totalTime)
		}
		barWidth := dialogWidth - 8
		if barWidth < 10 {
			barWidth = 10
		}
		filled := int(float64(barWidth) * pct)
		if filled < 0 {
			filled = 0
		}
		empty := barWidth - filled
		bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)

		// Color based on remaining time
		color := "\x1b[32m" // green
		if pct < 0.3 {
			color = "\x1b[31m" // red
		} else if pct < 0.6 {
			color = "\x1b[33m" // yellow
		}

		timeStr := formatDuration(c.remaining)
		lines = append(lines, fmt.Sprintf("│  %s%s%s\x1b[0m  %s%s│",
			color, bar, "\x1b[0m", timeStr,
			strings.Repeat(" ", dialogWidth-8-barWidth-len(timeStr))))

		lines = append(lines, "│" + strings.Repeat(" ", dialogWidth-2) + "│")

		// Hints
		hintLine := "  [Enter: confirm | Esc: dismiss]"
		lines = append(lines, fmt.Sprintf("│%s%s│", hintLine, strings.Repeat(" ", dialogWidth-2-len(hintLine))))
	}

	lines = append(lines, "└"+strings.Repeat("─", dialogWidth-2)+"┘")

	return lines
}

func (c *CountdownTimerComponent) HandleInput(data string) {
	if c.confirmed || c.dismissed {
		return
	}

	if tui.MatchesKey(data, "enter") {
		c.Confirm()
	} else if tui.MatchesKey(data, "escape") {
		c.Dismiss()
	}
}

func (c *CountdownTimerComponent) Invalidate()          {}
func (c *CountdownTimerComponent) WantsKeyRelease() bool { return false }

// TimedConfirm creates a countdown timer that auto-confirms or auto-dismisses.
func TimedConfirm(title, message string, timeout time.Duration, onResult func(confirmed bool)) *CountdownTimerComponent {
	tc := NewCountdownTimerComponent(title, message, timeout)
	tc.SetOnConfirm(func() { onResult(true) })
	tc.SetOnDismiss(func() { onResult(false) })
	tc.SetOnTimeout(func() { onResult(false) })
	return tc
}
