package codecui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/ai"
	"github.com/earendil-works/rho/pkg/tui"
)

// ToolExecutionState tracks the lifecycle of a single tool execution.
type ToolExecutionState string

const (
	ToolPending    ToolExecutionState = "pending"
	ToolRunning    ToolExecutionState = "running"
	ToolCompleted  ToolExecutionState = "completed"
	ToolFailed     ToolExecutionState = "failed"
	ToolBlocked    ToolExecutionState = "blocked"
)

// ToolExecutionComponent renders a single tool call and its result in the TUI.
type ToolExecutionComponent struct {
	toolCallID    string
	toolName      string
	args          map[string]interface{}
	state         ToolExecutionState
	output        string
	errorMessage  string
	isExpanded    bool
	startTime     time.Time
	elapsed       time.Duration
	mu            sync.Mutex
	onToggle      func()
	spinnerFrames []string
	spinnerIdx    int
}

// NewToolExecutionComponent creates a new tool execution display component.
func NewToolExecutionComponent(toolCallID, toolName string, args map[string]interface{}) *ToolExecutionComponent {
	return &ToolExecutionComponent{
		toolCallID:    toolCallID,
		toolName:      toolName,
		args:          args,
		state:         ToolPending,
		isExpanded:    true,
		startTime:     time.Now(),
		spinnerFrames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	}
}

// SetRunning transitions to running state.
func (tc *ToolExecutionComponent) SetRunning() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.state = ToolRunning
	tc.startTime = time.Now()
}

// SetOutput updates the output content and transitions to completed.
func (tc *ToolExecutionComponent) SetOutput(output string, isError bool) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if isError {
		tc.state = ToolFailed
		tc.errorMessage = output
	} else {
		tc.state = ToolCompleted
		tc.output = output
	}
	tc.elapsed = time.Since(tc.startTime)
}

// SetBlocked marks the tool as blocked by a hook.
func (tc *ToolExecutionComponent) SetBlocked(reason string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.state = ToolBlocked
	tc.errorMessage = reason
}

// ToggleExpanded toggles the expanded/collapsed state.
func (tc *ToolExecutionComponent) ToggleExpanded() {
	tc.mu.Lock()
	tc.isExpanded = !tc.isExpanded
	tc.mu.Unlock()
	if tc.onToggle != nil {
		tc.onToggle()
	}
}

// SetOnToggle sets the toggle callback.
func (tc *ToolExecutionComponent) SetOnToggle(fn func()) {
	tc.onToggle = fn
}

// Tick advances the spinner frame. Call periodically while running.
func (tc *ToolExecutionComponent) Tick() {
	tc.mu.Lock()
	tc.spinnerIdx = (tc.spinnerIdx + 1) % len(tc.spinnerFrames)
	tc.elapsed = time.Since(tc.startTime)
	tc.mu.Unlock()
}

func (tc *ToolExecutionComponent) Render(width int) []string {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if width <= 0 {
		return nil
	}

	var lines []string

	// Header line with state indicator
	header := tc.buildHeader(width)
	lines = append(lines, header)

	// Expanded content
	if tc.isExpanded {
		switch tc.state {
		case ToolPending:
			lines = append(lines, "  ⏳ Waiting...")
		case ToolRunning:
			// Show elapsed time
			elapsed := time.Since(tc.startTime).Round(time.Millisecond)
			elapsedStr := formatDuration(elapsed)
			spinner := tc.spinnerFrames[tc.spinnerIdx%len(tc.spinnerFrames)]
			lines = append(lines, fmt.Sprintf("  %s Running... (%s)", spinner, elapsedStr))
		case ToolCompleted:
			if tc.output != "" {
				outputLines := strings.Split(tc.output, "\n")
				maxOutput := 20
				if len(outputLines) > maxOutput {
					outputLines = outputLines[:maxOutput]
					outputLines = append(outputLines, fmt.Sprintf("  ... (%d more lines)", len(strings.Split(tc.output, "\n"))-maxOutput))
				}
				for _, l := range outputLines {
					truncated := l
					if len(truncated) > width-6 {
						truncated = truncated[:width-9] + "..."
					}
					lines = append(lines, "  │ "+truncated)
				}
				durationStr := formatDuration(tc.elapsed)
				lines = append(lines, fmt.Sprintf("  └── Completed in %s", durationStr))
			} else {
				lines = append(lines, "  ✓ Done (no output)")
			}
		case ToolFailed:
			errMsg := tc.errorMessage
			if errMsg == "" {
				errMsg = "Unknown error"
			}
			if len(errMsg) > width-8 {
				errMsg = errMsg[:width-11] + "..."
			}
			lines = append(lines, "  ✗ "+errMsg)
		case ToolBlocked:
			reason := tc.errorMessage
			if reason == "" {
				reason = "Blocked by extension"
			}
			lines = append(lines, "  ⊘ "+reason)
		}
	}

	return lines
}

func (tc *ToolExecutionComponent) buildHeader(width int) string {
	stateIcon := map[ToolExecutionState]string{
		ToolPending:   "○",
		ToolRunning:   "◌",
		ToolCompleted: "●",
		ToolFailed:    "●",
		ToolBlocked:   "⊘",
	}[tc.state]
	stateColor := map[ToolExecutionState]string{
		ToolPending:   "\x1b[2m",  // dim
		ToolRunning:   "\x1b[36m", // cyan
		ToolCompleted: "\x1b[32m", // green
		ToolFailed:    "\x1b[31m", // red
		ToolBlocked:   "\x1b[33m", // yellow
	}[tc.state]
	reset := "\x1b[0m"

	toolLabel := fmt.Sprintf("%s %s%s%s %s", stateIcon, stateColor, tc.toolName, reset, truncateArgs(tc.args))
	if len(toolLabel) > width-2 {
		toolLabel = toolLabel[:width-5] + "..."
	}

	expandHint := ""
	if tc.state == ToolCompleted || tc.state == ToolFailed {
		if tc.isExpanded {
			expandHint = " \x1b[2m[▲]\x1b[0m"
		} else {
			expandHint = " \x1b[2m[▼]\x1b[0m"
		}
	}

	return fmt.Sprintf("  %s%s", toolLabel, expandHint)
}

func (tc *ToolExecutionComponent) HandleInput(data string) {
	if tui.MatchesKey(data, "enter") || tui.MatchesKey(data, "space") {
		tc.ToggleExpanded()
	}
}

func (tc *ToolExecutionComponent) Invalidate() {}

func (tc *ToolExecutionComponent) WantsKeyRelease() bool { return false }

func truncateArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return "{}"
	}
	parts := make([]string, 0, len(args))
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	joined := strings.Join(parts, ", ")
	if len(joined) > 60 {
		joined = joined[:57] + "..."
	}
	return "[" + joined + "]"
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", m, s)
}

// ToolExecutionGroup renders multiple tool execution components as a group.
type ToolExecutionGroup struct {
	executions []*ToolExecutionComponent
	mu         sync.Mutex
}

// NewToolExecutionGroup creates a new tool execution group.
func NewToolExecutionGroup() *ToolExecutionGroup {
	return &ToolExecutionGroup{}
}

// AddExecution adds a tool execution component.
func (g *ToolExecutionGroup) AddExecution(exec *ToolExecutionComponent) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.executions = append(g.executions, exec)
}

// GetExecution returns an execution by tool call ID.
func (g *ToolExecutionGroup) GetExecution(toolCallID string) *ToolExecutionComponent {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, exec := range g.executions {
		if exec.toolCallID == toolCallID {
			return exec
		}
	}
	return nil
}

// Clear removes all executions.
func (g *ToolExecutionGroup) Clear() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.executions = nil
}

func (g *ToolExecutionGroup) Render(width int) []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	var lines []string
	for _, exec := range g.executions {
		execLines := exec.Render(width)
		lines = append(lines, execLines...)
	}
	return lines
}

func (g *ToolExecutionGroup) HandleInput(data string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, exec := range g.executions {
		exec.HandleInput(data)
	}
}

func (g *ToolExecutionGroup) Invalidate() {}
func (g *ToolExecutionGroup) WantsKeyRelease() bool { return false }

// Ensure types are used
var _ = agent.AgentToolResult{}
var _ = ai.Usage{}
