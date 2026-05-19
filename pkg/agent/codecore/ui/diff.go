package codecui

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/earendil-works/rho/pkg/tui"
)

// DiffRenderer renders side-by-side diffs with syntax highlighting.
type DiffRenderer struct {
	oldText string
	newText string
	context int
}

// NewDiffRenderer creates a diff renderer.
func NewDiffRenderer(oldText, newText string) *DiffRenderer {
	return &DiffRenderer{
		oldText: oldText,
		newText: newText,
		context: 3,
	}
}

// SetContext sets the context lines for unified diff.
func (dr *DiffRenderer) SetContext(n int) {
	dr.context = n
}

func (dr *DiffRenderer) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	reset := "\x1b[0m"
	green := "\x1b[32m"
	red := "\x1b[31m"
	dim := "\x1b[2m"
	bold := "\x1b[1m"
	cyan := "\x1b[36m"

	var lines []string

	// Try to use external diff tool, falling back to internal
	diffOutput := dr.computeDiff()

	if diffOutput == "" {
		return []string{dim + "  (no changes)" + reset}
	}

	scanner := bufio.NewScanner(strings.NewReader(diffOutput))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		switch line[0] {
		case '@':
			// Hunk header
			lines = append(lines, cyan+line+reset)
		case '+':
			content := line[1:] // strip +
			if tui.VisibleWidth(content) > width-2 {
				content = tui.SliceByColumn(content, 0, width-2, true)
			}
			lines = append(lines, green+"+"+content+reset)
		case '-':
			content := line[1:] // strip -
			if tui.VisibleWidth(content) > width-2 {
				content = tui.SliceByColumn(content, 0, width-2, true)
			}
			lines = append(lines, red+"-"+content+reset)
		default:
			if tui.VisibleWidth(line) > width {
				line = tui.SliceByColumn(line, 0, width, true)
			}
			lines = append(lines, dim+line+reset)
		}
	}

	// Add stats
	additions := strings.Count(diffOutput, "\n+") - strings.Count(diffOutput, "\n+++")
	deletions := strings.Count(diffOutput, "\n-") - strings.Count(diffOutput, "\n---")
	stats := fmt.Sprintf("%d additions, %d deletions", additions, deletions)
	lines = append(lines, bold+stats+reset)

	return lines
}

func (dr *DiffRenderer) computeDiff() string {
	// Try using external diff tool
	diffCmd := exec.Command("diff", "-u", "-", "-")
	stdin, _ := diffCmd.StdinPipe()

	go func() {
		defer stdin.Close()
		// Send old text, then new text separated by diff marker
		stdin.Write([]byte(dr.oldText))
	}()

	output, err := diffCmd.Output()
	if err == nil && len(output) > 0 {
		return string(output)
	}

	// Fallback: use internal line-based diff
	return internalDiff(dr.oldText, dr.newText)
}

// internalDiff computes a simple line-based diff.
func internalDiff(oldText, newText string) string {
	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")

	if len(oldLines) == 1 && oldLines[0] == "" && len(newLines) == 1 && newLines[0] == "" {
		return ""
	}

	var buf bytes.Buffer

	// Simple LCS-based diff
	m := len(oldLines)
	n := len(newLines)

	// Build LCS table
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to produce diff
	var diffLines []string
	i, j := m, n
	var oldStack, newStack []string

	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			// Common line
			for k := len(newStack) - 1; k >= 0; k-- {
				diffLines = append(diffLines, "+ "+newStack[k])
			}
			for k := len(oldStack) - 1; k >= 0; k-- {
				diffLines = append(diffLines, "- "+oldStack[k])
			}
			newStack = nil
			oldStack = nil
			diffLines = append(diffLines, "  "+oldLines[i-1])
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			newStack = append(newStack, newLines[j-1])
			j--
		} else if i > 0 {
			oldStack = append(oldStack, oldLines[i-1])
			i--
		}
	}

	for k := len(newStack) - 1; k >= 0; k-- {
		diffLines = append(diffLines, "+ "+newStack[k])
	}
	for k := len(oldStack) - 1; k >= 0; k-- {
		diffLines = append(diffLines, "- "+oldStack[k])
	}

	// Reverse to get correct order
	for i := len(diffLines) - 1; i >= 0; i-- {
		buf.WriteString(diffLines[i])
		buf.WriteString("\n")
	}

	return buf.String()
}

func (dr *DiffRenderer) HandleInput(data string) {}
func (dr *DiffRenderer) Invalidate()            {}
func (dr *DiffRenderer) WantsKeyRelease() bool  { return false }

// VisualTruncate provides smart truncation with show-more toggle.
type VisualTruncate struct {
	content   string
	maxLines  int
	maxChars  int
	expanded  bool
	truncated bool
}

// NewVisualTruncate creates smart truncated content.
func NewVisualTruncate(content string, maxLines, maxChars int) *VisualTruncate {
	return &VisualTruncate{
		content:  content,
		maxLines: maxLines,
		maxChars: maxChars,
	}
}

// IsTruncated returns whether content was truncated.
func (vt *VisualTruncate) IsTruncated() bool {
	return vt.truncated
}

// Toggle expands or collapses.
func (vt *VisualTruncate) Toggle() {
	vt.expanded = !vt.expanded
}

func (vt *VisualTruncate) Render(width int) []string {
	if width <= 0 || vt.content == "" {
		return nil
	}

	dim := "\x1b[2m"
	reset := "\x1b[0m"
	cyan := "\x1b[36m"

	lines := strings.Split(vt.content, "\n")

	if !vt.expanded && len(lines) > vt.maxLines {
		lines = lines[:vt.maxLines]
		vt.truncated = true
		lines = append(lines, dim+"... ("+cyan+fmt.Sprintf("%d more lines", len(strings.Split(vt.content, "\n"))-vt.maxLines)+dim+")"+reset)
	}

	if !vt.expanded && len(vt.content) > vt.maxChars {
		vt.truncated = true
		truncated := vt.content
		if len(truncated) > vt.maxChars {
			truncated = truncated[:vt.maxChars] + "..."
		}
		lines = []string{truncated}
		if len(strings.Split(vt.content, "\n")) > vt.maxLines {
			lines = append(lines, dim+"... ("+cyan+fmt.Sprintf("%d chars total", len(vt.content))+dim+")"+reset)
		}
	}

	// Truncate each line to width
	for i, l := range lines {
		if tui.VisibleWidth(l) > width {
			lines[i] = tui.SliceByColumn(l, 0, width, true)
		}
	}

	return lines
}

func (vt *VisualTruncate) HandleInput(data string) {
	if tui.MatchesKey(data, "enter") {
		vt.Toggle()
	}
}

func (vt *VisualTruncate) Invalidate()            {}
func (vt *VisualTruncate) WantsKeyRelease() bool  { return false }

// truncateToVisualLines truncates content to a specific number of visual lines.
func truncateToVisualLines(content string, maxLines int, width int) []string {
	if maxLines <= 0 || width <= 0 {
		return nil
	}

	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return lines
	}

	return lines[:maxLines]
}
