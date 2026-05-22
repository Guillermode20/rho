// Package agentutils provides shared utility functions for the rho coding agent.
package agentutils

import (
	"regexp"
	"strings"
)

var (
	ansiPattern    = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	oscPattern     = regexp.MustCompile(`\x1b\][0-9;]*(?:\x1b\\|\x07)`)
	hyperlinkAnsi  = regexp.MustCompile(`\x1b\]8;;(?:[^\x1b]*)\x1b\\`)
	sgrPattern     = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	cursorPattern  = regexp.MustCompile(`\x1b\[[0-9;]*[ABCDEFGHJKST]`)
	erasePattern   = regexp.MustCompile(`\x1b\[[0-9;]*[JK]`)
	kittyImageAnsi = regexp.MustCompile(`^\x1b_G`)
)

// StripANSI removes all ANSI escape sequences from a string.
func StripANSI(s string) string {
	s = ansiPattern.ReplaceAllString(s, "")
	s = oscPattern.ReplaceAllString(s, "")
	return s
}

// StripSGR removes only SGR (Select Graphic Rendition) sequences, preserving cursor movement and other controls.
func StripSGR(s string) string {
	return sgrPattern.ReplaceAllString(s, "")
}

// CategorizeANSI categorizes a line of terminal output by ANSI content type.
type ANSICategory int

const (
	ANSIPlain     ANSICategory = iota
	ANSISGR                    // Contains SGR color/style codes
	ANSICursor                 // Contains cursor movement codes
	ANSIErase                  // Contains erase codes
	ANSIHyperlink              // Contains hyperlink OSC 8 codes
	ANSIImage                  // Contains Kitty image protocol
	ANSIOther                  // Other ANSI sequences
)

// CategorizeANSI determines the category of ANSI sequences present in a string.
func CategorizeANSI(s string) ANSICategory {
	if kittyImageAnsi.MatchString(s) {
		return ANSIImage
	}
	if hyperlinkAnsi.MatchString(s) {
		return ANSIHyperlink
	}
	if sgrPattern.MatchString(s) {
		return ANSISGR
	}
	if cursorPattern.MatchString(s) {
		return ANSICursor
	}
	if erasePattern.MatchString(s) {
		return ANSIErase
	}
	if ansiPattern.MatchString(s) {
		return ANSIOther
	}
	return ANSIPlain
}

// ExtractHyperlinks extracts hyperlink URLs and their visible text from ANSI OSC 8 sequences.
// Returns a slice of {text, url} pairs.
func ExtractHyperlinks(s string) []struct{ Text, URL string } {
	var links []struct{ Text, URL string }
	re := regexp.MustCompile(`\x1b\]8;;(?:url=)?([^\x1b]*)\x1b\\([^\x1b]*)\x1b\]8;;\x07`)
	matches := re.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		if len(m) >= 3 {
			links = append(links, struct{ Text, URL string }{Text: m[2], URL: m[1]})
		}
	}
	return links
}

// WrapANSI wraps text respecting ANSI sequence boundaries. Ensures ANSI sequences
// are preserved and closed properly after wrapping.
func WrapANSI(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	segments := splitAnsiPreserving(text)
	var out strings.Builder
	lineWidth := 0
	for _, seg := range segments {
		if seg.isANSI {
			out.WriteString(seg.text)
			continue
		}
		for _, r := range seg.text {
			w := runeWidthANSI(r)
			if lineWidth+w > maxWidth {
				out.WriteString("\n")
				lineWidth = 0
			}
			out.WriteRune(r)
			lineWidth += w
		}
	}
	return out.String()
}

type ansiSeg struct {
	text   string
	isANSI bool
}

func splitAnsiPreserving(s string) []ansiSeg {
	var segs []ansiSeg
	loc := ansiPattern.FindStringIndex(s)
	last := 0
	for loc != nil {
		if loc[0] > last {
			segs = append(segs, ansiSeg{text: s[last:loc[0]], isANSI: false})
		}
		segs = append(segs, ansiSeg{text: s[loc[0]:loc[1]], isANSI: true})
		last = loc[1]
		loc = ansiPattern.FindStringIndex(s[last:])
		if loc != nil {
			loc[0] += last
			loc[1] += last
		}
	}
	if last < len(s) {
		segs = append(segs, ansiSeg{text: s[last:], isANSI: false})
	}
	return segs
}

func runeWidthANSI(r rune) int {
	if r < 32 || r == 0x7f {
		return 0
	}
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
	if r >= 0x1f1e0 && r <= 0x1f1ff {
		return 2
	}
	return 1
}

// HasANSI returns true if the string contains any ANSI escape sequences.
func HasANSI(s string) bool {
	return ansiPattern.MatchString(s) || oscPattern.MatchString(s)
}

// ANSILength returns the visible (non-ANSI) length of a string.
func ANSILength(s string) int {
	return len([]rune(StripANSI(s)))
}
