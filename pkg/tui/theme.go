package tui

import (
	"fmt"
	"strings"
)

// ─── Design System ──────────────────────────────────────────────────────────
//
// Theme defines a cohesive set of colors, spacing, and typography helpers.
// All TUI components use this — no more hardcoded ANSI codes.
//
// The design is minimalist: subtle neutrals, a single accent hue, lots of
// breathing room, and thin borders.

// Color represents a terminal color. It can be an ANSI 256‑color number
// or a hex string (e.g. "#ffffff").
type Color struct {
	ANSI int
	Hex  string
}

// ParseHexColor parses a hex color string (e.g., "#ffffff", "#fff", "ff0000") into RGB values.
func ParseHexColor(s string) (r, g, b int, err error) {
	s = strings.TrimPrefix(s, "#")
	if len(s) == 3 {
		format := "%1x%1x%1x"
		var r8, g8, b8 int
		_, err = fmt.Sscanf(s, format, &r8, &g8, &b8)
		if err != nil {
			return 0, 0, 0, err
		}
		return r8 * 17, g8 * 17, b8 * 17, nil
	} else if len(s) == 6 {
		format := "%02x%02x%02x"
		_, err = fmt.Sscanf(s, format, &r, &g, &b)
		return r, g, b, err
	}
	return 0, 0, 0, fmt.Errorf("invalid hex color: %s", s)
}

// Palette groups named ANSI/Hex color values for the whole UI.
type Palette struct {
	// Core
	Bg         Color // base background
	Fg         Color // default foreground
	Accent     Color // interactive / focused elements
	AccentAlt  Color // secondary accent (e.g. assistant name)
	Success    Color // success / user messages
	Warning    Color // warnings
	Error      Color // errors
	Dim        Color // muted / secondary text
	Border     Color // border lines
	Highlight  Color // selection highlight bg
	Surface    Color // card / panel bg (slightly lighter than Bg)
	SurfaceAlt Color // alternate surface bg
}

// DefaultPalette returns the default light‑on‑dark palette.
func DefaultPalette() Palette {
	return Palette{
		Bg:         Color{ANSI: 233}, // near‑black
		Fg:         Color{ANSI: 252}, // soft white
		Accent:     Color{ANSI: 75},  // blue
		AccentAlt:  Color{ANSI: 120}, // green
		Success:    Color{ANSI: 120}, // green
		Warning:    Color{ANSI: 214}, // amber
		Error:      Color{ANSI: 196}, // red
		Dim:        Color{ANSI: 241}, // medium gray
		Border:     Color{ANSI: 237}, // subtle border
		Highlight:  Color{ANSI: 235}, // highlight bg
		Surface:    Color{ANSI: 234}, // panel bg
		SurfaceAlt: Color{ANSI: 235}, // alternate surface
	}
}

// ─── Spacing ───────────────────────────────────────────────────────────────

type Spacing struct {
	XS    int //  1
	Small int //  2
	Med   int //  4
	Large int //  8
	Xl    int // 16
}

// DefaultSpacing returns the standard spacing scale.
func DefaultSpacing() Spacing {
	return Spacing{XS: 1, Small: 2, Med: 4, Large: 8, Xl: 16}
}

// ─── Border Style ──────────────────────────────────────────────────────────

type BorderStyle int

const (
	BorderMinimal BorderStyle = iota // subtle underline / overline only
	BorderLight                      // thin box chars (─ │)
	BorderHeavy                      // full rounded box (┌ ┐ └ ┘)
)

// ─── Theme ──────────────────────────────────────────────────────────────────

type Theme struct {
	Palette Palette
	Spacing Spacing

	// Border style preference for panels / boxes.
	Border BorderStyle

	// ── pre‑computed ANSI helpers ──
	ansi struct {
		reset string

		bg    func(Color) string
		fg    func(Color) string
		bold  string
		dim   string
		italic string
	}
}

// NewTheme builds a Theme from a palette, pre‑computing ANSI strings.
func NewTheme(p Palette) *Theme {
	t := &Theme{Palette: p, Spacing: DefaultSpacing(), Border: BorderLight}
	t.ansi.reset = "\x1b[0m"
	t.ansi.bg = func(c Color) string {
		if c.Hex != "" {
			if r, g, b, err := ParseHexColor(c.Hex); err == nil {
				return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
			}
		}
		return fmt.Sprintf("\x1b[48;5;%dm", c.ANSI)
	}
	t.ansi.fg = func(c Color) string {
		if c.Hex != "" {
			if r, g, b, err := ParseHexColor(c.Hex); err == nil {
				return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
			}
		}
		return fmt.Sprintf("\x1b[38;5;%dm", c.ANSI)
	}
	t.ansi.bold = "\x1b[1m"
	t.ansi.dim = "\x1b[2m"
	t.ansi.italic = "\x1b[3m"
	return t
}

// ── Convenience accessors ──

func (t *Theme) Reset() string          { return t.ansi.reset }
func (t *Theme) Bold(s string) string   { return t.ansi.bold + s + t.ansi.reset }
func (t *Theme) Dim(s string) string    { return t.ansi.dim + s + t.ansi.reset }
func (t *Theme) Italic(s string) string { return t.ansi.italic + s + t.ansi.reset }

func (t *Theme) Colored(s string, c Color) string { return t.ansi.fg(c) + s + t.ansi.reset }
func (t *Theme) Bg(s string, c Color) string      { return t.ansi.bg(c) + s + t.ansi.reset }

func (t *Theme) Accent(s string) string    { return t.Colored(s, t.Palette.Accent) }
func (t *Theme) AccentAlt(s string) string { return t.Colored(s, t.Palette.AccentAlt) }
func (t *Theme) Success(s string) string   { return t.Colored(s, t.Palette.Success) }
func (t *Theme) Warning(s string) string   { return t.Colored(s, t.Palette.Warning) }
func (t *Theme) Error(s string) string     { return t.Colored(s, t.Palette.Error) }
func (t *Theme) Muted(s string) string     { return t.Colored(s, t.Palette.Dim) }

// BoldAccent returns bold + accent.
func (t *Theme) BoldAccent(s string) string { return t.ansi.bold + t.ansi.fg(t.Palette.Accent) + s + t.ansi.reset }

// BoldAccentAlt returns bold + accent-alt.
func (t *Theme) BoldAccentAlt(s string) string { return t.ansi.bold + t.ansi.fg(t.Palette.AccentAlt) + s + t.ansi.reset }

// ── Borders ──

// TopBorder returns the top border line for a panel.
func (t *Theme) TopBorder(title string, width int) string {
	switch t.Border {
	case BorderMinimal:
		return t.Muted(strings.Repeat("─", width))
	case BorderLight:
		line := "─" + strings.Repeat("─", max(0, width-2)) + "─"
		return t.Muted(line)
	default: // BorderHeavy
		if title != "" {
			titleMax := width - 4
			disp := title
			if len(disp) > titleMax {
				disp = disp[:titleMax]
			}
			rightDashes := max(0, width-4-len(disp)-2)
			return t.Muted("┌─ " + disp + " " + strings.Repeat("─", rightDashes) + "┐")
		}
		return t.Muted("┌" + strings.Repeat("─", width-2) + "┐")
	}
}

// BottomBorder returns the bottom border line.
func (t *Theme) BottomBorder(width int) string {
	switch t.Border {
	case BorderMinimal:
		return ""
	case BorderLight:
		return t.Muted("─" + strings.Repeat("─", max(0, width-2)) + "─")
	default:
		return t.Muted("└" + strings.Repeat("─", width-2) + "┘")
	}
}

// SideBorder wraps content with side borders.
func (t *Theme) SideBorder(line string, width int) string {
	if t.Border == BorderMinimal {
		return line
	}
	return t.Muted("│") + line[:min(len(line), width-2)] + t.Muted("│")
}

// ── Separator ──

func (t *Theme) Separator(width int) string {
	return t.Muted(strings.Repeat("─", width))
}

// ── Label / Tag helpers ──

// Tag renders a compact label like "[model/provider]".
func (t *Theme) Tag(text string) string {
	return t.Muted("[" + text + "]")
}

// ── Default instance ──

var DefaultTheme = NewTheme(DefaultPalette())
