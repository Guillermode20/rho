package tui

import (
	"fmt"
	"regexp"
	"strings"
)

// MarkdownTheme defines styling for Markdown rendering.
type MarkdownTheme struct {
	H1          func(text string) string
	H2          func(text string) string
	H3          func(text string) string
	Bold        func(text string) string
	Italic      func(text string) string
	Code        func(text string) string
	CodeBlock   func(text string) string
	Link        func(text, url string) string
	Blockquote  func(text string) string
	ListBullet  func(text string) string
	ListNumber  func(text string) string
	Horizontal  func() string
}

// DefaultMarkdownTheme returns a markdown theme using the design system palette.
func DefaultMarkdownTheme() MarkdownTheme {
	th := DefaultTheme
	reset := th.Reset()
	bold := th.ansi.bold
	dim := th.ansi.dim
	fg75 := th.ansi.fg(Color{ANSI: 75})  // accent blue for headings
	fg120 := th.ansi.fg(Color{ANSI: 120}) // green for code
	fg221 := th.ansi.fg(Color{ANSI: 221}) // amber for inline code
	fg242 := th.ansi.fg(Color{ANSI: 242}) // dim gray for metadata

	return MarkdownTheme{
		H1: func(text string) string { return bold + fg75 + text + reset },
		H2: func(text string) string { return bold + fg75 + text + reset },
		H3: func(text string) string { return bold + fg75 + text + reset },
		Bold: func(text string) string { return bold + text + reset },
		Italic: func(text string) string { return dim + text + reset },
		Code: func(text string) string { return fg221 + text + reset },
		CodeBlock: func(text string) string { return fg120 + text + reset },
		Link: func(text, url string) string { return fg75 + text + reset + fg242 + " (" + url + ")" + reset },
		Blockquote: func(text string) string { return dim + "▎" + text + reset },
		ListBullet: func(text string) string { return "• " + text },
		ListNumber: func(text string) string { return text },
		Horizontal: func() string { return dim + strings.Repeat("─", 40) + reset },
	}
}

// Markdown is a component that renders Markdown content.
type Markdown struct {
	content string
	theme   MarkdownTheme
}

// NewMarkdown creates a new Markdown component.
func NewMarkdown(content string, theme MarkdownTheme) *Markdown {
	return &Markdown{content: content, theme: theme}
}

// SetContent updates the markdown content.
func (m *Markdown) SetContent(content string) {
	m.content = content
}

func (m *Markdown) Render(width int) []string {
	if width <= 0 || m.content == "" {
		return nil
	}
	return m.renderMarkdown(m.content, width)
}

func (m *Markdown) HandleInput(data string) {}
func (m *Markdown) Invalidate()            {}
func (m *Markdown) WantsKeyRelease() bool  { return false }

// renderMarkdown converts markdown text to rendered terminal lines.
func (m *Markdown) renderMarkdown(text string, width int) []string {
	var lines []string

	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")

	// Split into blocks
	blocks := splitMarkdownBlocks(text)

	for _, block := range blocks {
		switch block.kind {
		case "h1":
			content := strings.TrimPrefix(block.content, "# ")
			lines = append(lines, "")
			lines = append(lines, m.theme.H1(content))
			lines = append(lines, m.theme.H1(strings.Repeat("═", VisibleWidth(content))))
			lines = append(lines, "")

		case "h2":
			content := strings.TrimPrefix(block.content, "## ")
			lines = append(lines, "")
			lines = append(lines, m.theme.H2(content))
			lines = append(lines, m.theme.H2(strings.Repeat("─", VisibleWidth(content))))
			lines = append(lines, "")

		case "h3":
			content := strings.TrimPrefix(block.content, "### ")
			lines = append(lines, "")
			lines = append(lines, m.theme.H3(content))
			lines = append(lines, "")

		case "code":
			codeLines := strings.Split(block.content, "\n")
			for _, cl := range codeLines {
				lines = append(lines, m.theme.CodeBlock("  "+cl))
			}
			lines = append(lines, "")

		case "blockquote":
			for _, bl := range strings.Split(block.content, "\n") {
				trimmed := strings.TrimPrefix(bl, "> ")
				trimmed = strings.TrimPrefix(trimmed, ">")
				lines = append(lines, m.theme.Blockquote(trimmed))
			}

		case "list":
			for i, item := range block.items {
				prefix := m.theme.ListBullet("")
				if block.ordered {
					prefix = fmt.Sprintf("%d. ", i+1)
				}
				rendered := m.renderInline(item, width-len(prefix))
				for j, rl := range rendered {
					if j == 0 {
						lines = append(lines, prefix+rl)
					} else {
						lines = append(lines, strings.Repeat(" ", VisibleWidth(prefix))+rl)
					}
				}
			}
			lines = append(lines, "")

		case "hr":
			lines = append(lines, "")
			lines = append(lines, m.theme.Horizontal())
			lines = append(lines, "")

		case "paragraph":
			rendered := m.renderInline(block.content, width)
			lines = append(lines, rendered...)
			lines = append(lines, "")
		}
	}

	return lines
}

type mdBlock struct {
	kind    string // h1, h2, h3, code, blockquote, list, hr, paragraph
	content string
	items   []string
	ordered bool
}

func splitMarkdownBlocks(text string) []mdBlock {
	var blocks []mdBlock
	lines := strings.Split(text, "\n")
	i := 0

	for i < len(lines) {
		line := lines[i]

		// Code block
		if strings.HasPrefix(line, "```") {
			var codeLines []string
			i++
			for i < len(lines) && !strings.HasPrefix(lines[i], "```") {
				codeLines = append(codeLines, lines[i])
				i++
			}
			i++ // skip closing ```
			blocks = append(blocks, mdBlock{kind: "code", content: strings.Join(codeLines, "\n")})
			continue
		}

		// Headings
		if strings.HasPrefix(line, "### ") {
			blocks = append(blocks, mdBlock{kind: "h3", content: line})
			i++
			continue
		}
		if strings.HasPrefix(line, "## ") {
			blocks = append(blocks, mdBlock{kind: "h2", content: line})
			i++
			continue
		}
		if strings.HasPrefix(line, "# ") {
			blocks = append(blocks, mdBlock{kind: "h1", content: line})
			i++
			continue
		}

		// Blockquote
		if strings.HasPrefix(line, "> ") || strings.HasPrefix(line, ">") {
			var quoteLines []string
			for i < len(lines) && (strings.HasPrefix(lines[i], "> ") || strings.HasPrefix(lines[i], ">")) {
				quoteLines = append(quoteLines, lines[i])
				i++
			}
			blocks = append(blocks, mdBlock{kind: "blockquote", content: strings.Join(quoteLines, "\n")})
			continue
		}

		// Horizontal rule
		if matched, _ := regexp.MatchString(`^[-*_]{3,}\s*$`, line); matched {
			blocks = append(blocks, mdBlock{kind: "hr"})
			i++
			continue
		}

		// List items
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			var items []string
			for i < len(lines) && (strings.HasPrefix(lines[i], "- ") || strings.HasPrefix(lines[i], "* ")) {
				items = append(items, strings.TrimPrefix(strings.TrimPrefix(lines[i], "- "), "* "))
				i++
			}
			blocks = append(blocks, mdBlock{kind: "list", items: items, ordered: false})
			continue
		}

		// Ordered list
		if matched, _ := regexp.MatchString(`^\d+\.\s`, line); matched {
			var items []string
			for i < len(lines) {
				matched2, _ := regexp.MatchString(`^\d+\.\s`, lines[i])
				if !matched2 {
					break
				}
				re := regexp.MustCompile(`^\d+\.\s`)
				items = append(items, re.ReplaceAllString(lines[i], ""))
				i++
			}
			blocks = append(blocks, mdBlock{kind: "list", items: items, ordered: true})
			continue
		}

		// Empty line - skip
		if strings.TrimSpace(line) == "" {
			i++
			continue
		}

		// Paragraph (collect consecutive non-empty lines)
		var paraLines []string
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" &&
			!strings.HasPrefix(lines[i], "#") &&
			!strings.HasPrefix(lines[i], "```") &&
			!strings.HasPrefix(lines[i], "> ") &&
			!strings.HasPrefix(lines[i], ">") &&
			!strings.HasPrefix(lines[i], "- ") &&
			!strings.HasPrefix(lines[i], "* ") {
			matched3, _ := regexp.MatchString(`^\d+\.\s`, lines[i])
			if matched3 {
				break
			}
			paraLines = append(paraLines, lines[i])
			i++
		}
		if len(paraLines) > 0 {
			blocks = append(blocks, mdBlock{kind: "paragraph", content: strings.Join(paraLines, " ")})
		}
	}

	return blocks
}

// Inline rendering patterns
var (
	boldRe      = regexp.MustCompile(`\*\*(.+?)\*\*`)
	italicRe    = regexp.MustCompile(`\*(.+?)\*`)
	codeInlineRe = regexp.MustCompile("`(.+?)`")
	linkRe       = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
)

func (m *Markdown) renderInline(text string, width int) []string {
	if width <= 0 {
		width = 80
	}

	// Apply inline formatting
	formatted := text
	formatted = boldRe.ReplaceAllStringFunc(formatted, func(match string) string {
		inner := boldRe.FindStringSubmatch(match)[1]
		return m.theme.Bold(inner)
	})
	formatted = italicRe.ReplaceAllStringFunc(formatted, func(match string) string {
		inner := italicRe.FindStringSubmatch(match)[1]
		return m.theme.Italic(inner)
	})
	formatted = codeInlineRe.ReplaceAllStringFunc(formatted, func(match string) string {
		inner := codeInlineRe.FindStringSubmatch(match)[1]
		return m.theme.Code(inner)
	})
	formatted = linkRe.ReplaceAllStringFunc(formatted, func(match string) string {
		parts := linkRe.FindStringSubmatch(match)
		return m.theme.Link(parts[1], parts[2])
	})

	// Word-wrap
	return WrapTextWithAnsi(formatted, width)
}
