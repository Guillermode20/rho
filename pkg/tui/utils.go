package tui

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// ANSI escape sequence patterns for width calculations.
var (
	ansiPattern    = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	oscPattern     = regexp.MustCompile(`\x1b\][0-9;]*(?:\x1b\\|\x07)`)
	hyperlinkRe    = regexp.MustCompile(`\x1b\]8;;(?:[^\x1b]*)\x1b\\`)
	kittyImageRe   = regexp.MustCompile(`^\x1b_G`)
	cursorMarkerRe = regexp.MustCompile(`\x1b_pi:c\x07`)
)

// StripANSI removes ANSI escape sequences from a string.
func StripANSI(s string) string {
	s = ansiPattern.ReplaceAllString(s, "")
	s = oscPattern.ReplaceAllString(s, "")
	return s
}

// IsImageLine returns true if the line contains a Kitty image protocol sequence.
func IsImageLine(line string) bool {
	return strings.HasPrefix(line, "\x1b_G")
}

// NormalizeTerminalOutput ensures the terminal output is clean.
// This function strips cursor markers and normalizes line endings.
func NormalizeTerminalOutput(s string) string {
	s = cursorMarkerRe.ReplaceAllString(s, "")
	return s
}

// visibleWidth returns the visual display width of a string, accounting for
// ANSI escape sequences and wide characters (e.g., CJK, emoji).
func VisibleWidth(s string) int {
	width := 0
	strip := ansiPattern.ReplaceAllStringFunc(s, func(m string) string {
		return ""
	})
	strip = oscPattern.ReplaceAllString(strip, "")
	strip = cursorMarkerRe.ReplaceAllString(strip, "")

	for len(strip) > 0 {
		r, size := utf8.DecodeRuneInString(strip)
		if r == utf8.RuneError {
			strip = strip[size:]
			continue
		}
		width += runeWidth(r)
		strip = strip[size:]
	}
	return width
}

// runeWidth returns the display width of a rune (1 for narrow, 2 for wide).
func runeWidth(r rune) int {
	if r == 0 {
		return 0
	}
	if r < 32 || r == 0x7f {
		return 0
	}
	// East Asian Width ambiguous / wide ranges
	if r >= 0x1100 &&
		(r <= 0x115f || r == 0x2329 || r == 0x232a ||
			(r >= 0x2e80 && r <= 0x303e) ||
			(r >= 0x3040 && r <= 0x33ff) ||
			(r >= 0x3400 && r <= 0x4dbf) ||
			(r >= 0x4e00 && r <= 0xa4cf) ||
			(r >= 0xa960 && r <= 0xa97f) ||
			(r >= 0xac00 && r <= 0xd7af) ||
			(r >= 0xf900 && r <= 0xfaff) ||
			(r >= 0xfe10 && r <= 0xfe1f) ||
			(r >= 0xfe30 && r <= 0xfe6f) ||
			(r >= 0xff01 && r <= 0xff60) ||
			(r >= 0xffe0 && r <= 0xffef) ||
			(r >= 0x1b000 && r <= 0x1b0ff) ||
			(r >= 0x1d000 && r <= 0x1d0ff) ||
			(r >= 0x1f000 && r <= 0x1ffff) ||
			(r >= 0x20000 && r <= 0x2ffff) ||
			(r >= 0x30000 && r <= 0x3ffff) ||
			(r >= 0xe0100 && r <= 0xe01ef)) {
		return 2
	}
	// Regional indicator symbols (flags)
	if r >= 0x1f1e0 && r <= 0x1f1ff {
		return 2
	}
	return 1
}

// TruncateToWidth truncates a string (with ANSI) to the given visible width.
// Wide characters at the boundary are excluded to prevent visual overflow.
func TruncateToWidth(s string, maxWidth int) string {
	return SliceByColumn(s, 0, maxWidth, true)
}

// WrapTextWithAnsi wraps text with ANSI sequences to fit within a maximum width.
// It preserves ANSI sequences in the output.
func WrapTextWithAnsi(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return nil
	}

	// Split into ANSI and visible segments
	segments := splitAnsi(text)

	var lines []string
	var currentLine strings.Builder
	var currentWidth int

	for _, seg := range segments {
		if seg.isAnsi {
			currentLine.WriteString(seg.text)
			continue
		}

		runes := []rune(seg.text)
		for _, r := range runes {
			w := runeWidth(r)
			if currentWidth+w > maxWidth {
				if currentLine.Len() > 0 {
					lines = append(lines, currentLine.String())
					currentLine.Reset()
					currentWidth = 0
				}
				// If the word is wider than maxWidth, wrap at character level
				if w > maxWidth {
					lines = append(lines, string(r))
					continue
				}
			}
			currentLine.WriteRune(r)
			currentWidth += w
		}
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}

type ansiSegment struct {
	text   string
	isAnsi bool
}

// splitAnsi splits a string into visible and ANSI escape sequence segments.
func splitAnsi(s string) []ansiSegment {
	var segs []ansiSegment

	loc := ansiPattern.FindStringIndex(s)
	if loc == nil {
		return append(segs, ansiSegment{text: s, isAnsi: false})
	}

	last := 0
	for loc != nil {
		if loc[0] > last {
			segs = append(segs, ansiSegment{text: s[last:loc[0]], isAnsi: false})
		}
		segs = append(segs, ansiSegment{text: s[loc[0]:loc[1]], isAnsi: true})
		last = loc[1]
		loc = ansiPattern.FindStringIndex(s[last:])
		if loc != nil {
			loc[0] += last
			loc[1] += last
		}
	}

	if last < len(s) {
		segs = append(segs, ansiSegment{text: s[last:], isAnsi: false})
	}

	return segs
}

// SliceByColumn slices a string (possibly containing ANSI sequences) by visual column.
// Returns the substring that occupies columns [start, end) visually.
// If strict is true, wide characters at the boundary are excluded.
func SliceByColumn(s string, start, end int, strict bool) string {
	if start >= end || start < 0 {
		return ""
	}

	type runeInfo struct {
		r     rune
		width int
		ansi  string
	}

	scanner := make([]runeInfo, 0)
	remaining := s

	for len(remaining) > 0 {
		// Check for ANSI/OSC prefix
		ansiLoc := ansiPattern.FindStringIndex(remaining)
		oscLoc := oscPattern.FindStringIndex(remaining)

		nextSpecial := -1
		var specialStr string
		specialLen := 0

		if ansiLoc != nil && ansiLoc[0] == 0 {
			nextSpecial = 0
			specialStr = remaining[ansiLoc[0]:ansiLoc[1]]
			specialLen = ansiLoc[1]
		} else if oscLoc != nil && oscLoc[0] == 0 {
			nextSpecial = 0
			specialStr = remaining[oscLoc[0]:oscLoc[1]]
			specialLen = oscLoc[1]
		}

		if nextSpecial == 0 {
			scanner = append(scanner, runeInfo{ansi: specialStr})
			remaining = remaining[specialLen:]
			continue
		}

		r, size := utf8.DecodeRuneInString(remaining)
		if r == utf8.RuneError {
			remaining = remaining[size:]
			continue
		}
		scanner = append(scanner, runeInfo{r: r, width: runeWidth(r)})
		remaining = remaining[size:]
	}

	width := 0
	var result strings.Builder
	for _, ri := range scanner {
		if ri.ansi != "" {
			if width >= start && width < end {
				result.WriteString(ri.ansi)
			}
			continue
		}

		riEnd := width + ri.width
		if riEnd <= start {
			width = riEnd
			continue
		}
		if width >= end {
			break
		}
		if strict && riEnd > end {
			break
		}
		if width >= start {
			result.WriteRune(ri.r)
		}
		width = riEnd
	}

	return result.String()
}

// ExtractSegments extracts text before and after a given column range.
// Returns the text before [0, start), at [start, end), and after [end, ∞).
func ExtractSegments(s string, start, end, remainingAfter int, strict bool) (before, at, after string) {
	before = SliceByColumn(s, 0, start, strict)
	at = SliceByColumn(s, start, end, strict)
	after = SliceByColumn(s, end, end+remainingAfter, strict)
	return
}

// CursorMarker is a zero-width escape sequence for cursor positioning.
const CursorMarker = "\x1b_pi:c\x07"

// SegmentReset is the ANSI reset sequence used between composited segments.
const SegmentReset = "\x1b[0m\x1b]8;;\x07"
