package codecui

import "github.com/earendil-works/rho/pkg/tui"

// VisualTruncateResult contains truncated text info.
type VisualTruncateResult struct {
	Text      string
	Truncated bool
	Lines     int
}

// TruncateToVisualLines truncates text to a max number of visual lines at a given width.
func TruncateToVisualLines(text string, maxLines, width int) VisualTruncateResult {
	lines := tui.WrapTextWithAnsi(text, width)
	if len(lines) <= maxLines {
		return VisualTruncateResult{Text: text, Lines: len(lines)}
	}
	truncated := lines[:maxLines]
	last := truncated[maxLines-1]
	if tui.VisibleWidth(last) >= width-3 {
		last = tui.SliceByColumn(last, 0, width-3, true) + "..."
	}
	truncated[maxLines-1] = last
	result := ""
	for i, l := range truncated {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return VisualTruncateResult{Text: result, Truncated: true, Lines: maxLines}
}
