package codecui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/earendil-works/rho/pkg/tui"
)

// BashExecution displays a bash command execution with output.
type BashExecution struct {
	command   string
	exitCode  int
	stdout    string
	stderr    string
	output    string
	startTime time.Time
	duration  time.Duration
	running   bool
	truncated bool
	expanded  bool
}

// NewBashExecution creates a new bash execution component.
func NewBashExecution(command string) *BashExecution {
	return &BashExecution{
		command:   command,
		startTime: time.Now(),
		running:   true,
		expanded:  true,
	}
}

// SetCompleted marks the execution as completed.
func (be *BashExecution) SetCompleted(exitCode int, output string, truncated bool) {
	be.exitCode = exitCode
	be.output = output
	be.duration = time.Since(be.startTime)
	be.running = false
	be.truncated = truncated
}

// SetRunning updates the running state.
func (be *BashExecution) SetRunning(running bool) {
	be.running = running
}

func (be *BashExecution) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	reset := "\x1b[0m"
	red := "\x1b[31m"
	yellow := "\x1b[33m"
	dim := "\x1b[2m"
	bold := "\x1b[1m"
	cyan := "\x1b[36m"

	var lines []string

	header := bold + cyan + "\u276e " + reset + be.command
	if be.running {
		header += yellow + "  \u23f3 running..." + reset
	} else {
		durationStr := fmt.Sprintf("%.1fs", be.duration.Seconds())
		if be.exitCode != 0 {
			header += red + fmt.Sprintf("  [%s exit %d]", durationStr, be.exitCode) + reset
		} else {
			header += dim + fmt.Sprintf("  [%s exit %d]", durationStr, be.exitCode) + reset
		}
	}
	lines = append(lines, header)

	if be.output != "" && be.expanded {
		output := be.output
		maxLines := 20
		outputLines := strings.Split(output, "\n")
		if len(outputLines) > maxLines {
			outputLines = outputLines[:maxLines]
		}
		if be.truncated || len(outputLines) >= maxLines {
			outputLines = append(outputLines, dim+"... (output truncated)"+reset)
		}
		for _, l := range outputLines {
			if tui.VisibleWidth(l) > width-4 {
				l = tui.SliceByColumn(l, 0, width-4, true)
			}
			lines = append(lines, "  "+l)
		}
	} else if be.output != "" && !be.expanded {
		firstLine := strings.SplitN(be.output, "\n", 2)[0]
		if tui.VisibleWidth(firstLine) > width-8 {
			firstLine = tui.SliceByColumn(firstLine, 0, width-8, true) + "..."
		}
		lines = append(lines, dim+"  "+firstLine+reset)
	}

	if be.truncated && be.expanded {
		lines = append(lines, dim+"  [Output truncated at limit]"+reset)
	}

	return lines
}

func (be *BashExecution) HandleInput(data string) {
	if tui.MatchesKey(data, "enter") {
		be.expanded = !be.expanded
	}
}

func (be *BashExecution) Invalidate() {}
func (be *BashExecution) WantsKeyRelease() bool { return false }

// ToolExecution displays any tool execution with progress and result.
type ToolExecution struct {
	toolName  string
	args      string
	output    string
	isError   bool
	isRunning bool
	startTime time.Time
	duration  time.Duration
	expanded  bool
}

// NewToolExecution creates a new tool execution component.
func NewToolExecution(toolName string, args string) *ToolExecution {
	return &ToolExecution{
		toolName:  toolName,
		args:      args,
		startTime: time.Now(),
		isRunning: true,
		expanded:  true,
	}
}

// SetResult sets the tool execution result.
func (te *ToolExecution) SetResult(output string, isError bool) {
	te.output = output
	te.isError = isError
	te.duration = time.Since(te.startTime)
	te.isRunning = false
}

func (te *ToolExecution) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	reset := "\x1b[0m"
	green := "\x1b[32m"
	red := "\x1b[31m"
	yellow := "\x1b[33m"
	dim := "\x1b[2m"
	bold := "\x1b[1m"

	var lines []string

	var statusIcon string
	if te.isRunning {
		statusIcon = yellow + "\u23f3" + reset
	} else if te.isError {
		statusIcon = red + "\u2717" + reset
	} else {
		statusIcon = green + "\u2713" + reset
	}

	header := fmt.Sprintf("%s %s%s%s %s", statusIcon, bold, te.toolName, reset, te.args)
	if !te.isRunning {
		durationStr := fmt.Sprintf("%.1fs", te.duration.Seconds())
		header += dim + " (" + durationStr + ")" + reset
	}
	lines = append(lines, header)

	if te.output != "" && te.expanded {
		maxLines := 15
		outputLines := strings.Split(te.output, "\n")
		if len(outputLines) > maxLines {
			outputLines = outputLines[:maxLines]
			outputLines = append(outputLines, dim+"... (output truncated)"+reset)
		}
		for _, l := range outputLines {
			if tui.VisibleWidth(l) > width-4 {
				l = tui.SliceByColumn(l, 0, width-4, true)
			}
			lines = append(lines, "  "+l)
		}
	}

	return lines
}

func (te *ToolExecution) HandleInput(data string) {
	if tui.MatchesKey(data, "enter") {
		te.expanded = !te.expanded
	}
}

func (te *ToolExecution) Invalidate()            {}
func (te *ToolExecution) WantsKeyRelease() bool  { return false }

// ExpandableContent displays content that can be toggled between expanded/collapsed.
type ExpandableContent struct {
	content  []string
	expanded bool
	label    string
}

// NewExpandableContent creates expandable content.
func NewExpandableContent(label string, content []string) *ExpandableContent {
	return &ExpandableContent{
		content:  content,
		expanded: false,
		label:    label,
	}
}

func (ec *ExpandableContent) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	dim := "\x1b[2m"
	reset := "\x1b[0m"

	if ec.expanded {
		lines := []string{dim + "\u25bc " + ec.label + reset}
		for _, l := range ec.content {
			if tui.VisibleWidth(l) > width-2 {
				l = tui.SliceByColumn(l, 0, width-2, true)
			}
			lines = append(lines, " "+l)
		}
		return lines
	}

	preview := ec.label
	if len(ec.content) > 0 {
		firstLine := ec.content[0]
		if tui.VisibleWidth(firstLine) > width-len(preview)-10 {
			firstLine = tui.SliceByColumn(firstLine, 0, width-len(preview)-10, true)
		}
		preview += ": " + firstLine
	}
	return []string{dim + "\u25b6 " + preview + reset}
}

func (ec *ExpandableContent) HandleInput(data string) {
	if tui.MatchesKey(data, "enter") {
		ec.expanded = !ec.expanded
	}
}

func (ec *ExpandableContent) Invalidate()            {}
func (ec *ExpandableContent) WantsKeyRelease() bool  { return false }

// ConsoleBlock renders a block of console/shell output.
type ConsoleBlock struct {
	content   string
	truncated bool
}

// NewConsoleBlock creates a console output block.
func NewConsoleBlock(content string) *ConsoleBlock {
	return &ConsoleBlock{content: content}
}

// SetTruncated marks the block as truncated.
func (cb *ConsoleBlock) SetTruncated(t bool) {
	cb.truncated = t
}

func (cb *ConsoleBlock) Render(width int) []string {
	if width <= 0 || cb.content == "" {
		return nil
	}

	dim := "\x1b[2m"
	reset := "\x1b[0m"

	var lines []string
	contentLines := strings.Split(cb.content, "\n")

	maxLines := 25
	if len(contentLines) > maxLines {
		contentLines = contentLines[:maxLines]
	}

	for _, l := range contentLines {
		if tui.VisibleWidth(l) > width-2 {
			l = tui.SliceByColumn(l, 0, width-2, true)
		}
		lines = append(lines, " "+l)
	}

	if cb.truncated || len(contentLines) >= maxLines {
		lines = append(lines, dim+" ... (truncated)"+reset)
	}

	return lines
}

func (cb *ConsoleBlock) HandleInput(data string) {}
func (cb *ConsoleBlock) Invalidate()            {}
func (cb *ConsoleBlock) WantsKeyRelease() bool  { return false }

// SkillInvocationMessage displays a skill being invoked.
type SkillInvocationMessage struct {
	skillName string
	status    string
	output    string
}

// NewSkillInvocationMessage creates a new skill invocation display.
func NewSkillInvocationMessage(skillName string) *SkillInvocationMessage {
	return &SkillInvocationMessage{
		skillName: skillName,
		status:    "running",
	}
}

// SetStatus updates the skill status.
func (sim *SkillInvocationMessage) SetStatus(status, output string) {
	sim.status = status
	sim.output = output
}

func (sim *SkillInvocationMessage) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	reset := "\x1b[0m"
	green := "\x1b[32m"
	red := "\x1b[31m"
	yellow := "\x1b[33m"
	dim := "\x1b[2m"
	bold := "\x1b[1m"

	var icon string
	switch sim.status {
	case "running":
		icon = yellow + "\u25d0" + reset
	case "completed":
		icon = green + "\u2713" + reset
	case "error":
		icon = red + "\u2717" + reset
	}

	_ = dim
	lines := []string{fmt.Sprintf("%s %s%s%s", icon, bold, sim.skillName, reset)}

	if sim.output != "" {
		outputLines := strings.Split(sim.output, "\n")
		for _, l := range outputLines {
			if tui.VisibleWidth(l) > width-4 {
				l = tui.SliceByColumn(l, 0, width-4, true)
			}
			lines = append(lines, "  "+l)
		}
	}

	return lines
}

func (sim *SkillInvocationMessage) HandleInput(data string) {}
func (sim *SkillInvocationMessage) Invalidate()             {}
func (sim *SkillInvocationMessage) WantsKeyRelease() bool   { return false }

// ProgressBar renders a simple progress bar.
type ProgressBar struct {
	percent float64
	label   string
}

// NewProgressBar creates a progress bar component.
func NewProgressBar(label string) *ProgressBar {
	return &ProgressBar{label: label}
}

// SetPercent sets the progress percentage (0-100).
func (pb *ProgressBar) SetPercent(pct float64) {
	pb.percent = math.Max(0, math.Min(100, pct))
}

func (pb *ProgressBar) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	barWidth := width - len(pb.label) - 4
	if barWidth < 10 {
		barWidth = 10
	}

	filled := int(float64(barWidth) * pb.percent / 100.0)
	empty := barWidth - filled

	bar := "\x1b[32m" + strings.Repeat("\u2588", filled) + "\x1b[0m" +
		"\x1b[2m" + strings.Repeat("\u2591", empty) + "\x1b[0m"

	return []string{fmt.Sprintf("%s [%s] %.0f%%", pb.label, bar, pb.percent)}
}

func (pb *ProgressBar) HandleInput(data string) {}
func (pb *ProgressBar) Invalidate()            {}
func (pb *ProgressBar) WantsKeyRelease() bool  { return false }
