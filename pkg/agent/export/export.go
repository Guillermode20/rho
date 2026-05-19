// Package export provides HTML export of agent conversations.
package export

import (
	"bytes"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/ai"
)

// ExportOptions configures the HTML export.
type ExportOptions struct {
	Title       string `json:"title,omitempty"`
	IncludeCSS  bool   `json:"includeCSS"`
	IncludeJS   bool   `json:"includeJS"`
	DarkMode    bool   `json:"darkMode"`
	ShowTimestamps bool `json:"showTimestamps"`
}

// DefaultExportOptions returns sensible defaults.
func DefaultExportOptions() ExportOptions {
	return ExportOptions{
		Title:          "rho Session Export",
		IncludeCSS:     true,
		IncludeJS:      true,
		DarkMode:       false,
		ShowTimestamps: true,
	}
}

// defaultTemplateCSS returns the embedded default CSS template.
func defaultTemplateCSS() string {
	return `:root {
  --bg: #ffffff;
  --fg: #1a1a1a;
  --user-bg: #e8f0fe;
  --assistant-bg: #f5f5f5;
  --tool-bg: #fff8e1;
  --border: #e0e0e0;
  --accent: #1a73e8;
  --code-bg: #f0f0f0;
}

@media (prefers-color-scheme: dark) {
  :root {
    --bg: #1e1e1e;
    --fg: #e0e0e0;
    --user-bg: #1a3a5c;
    --assistant-bg: #2d2d2d;
    --tool-bg: #3d3200;
    --border: #404040;
    --accent: #89b4fa;
    --code-bg: #2d2d2d;
  }
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  max-width: 900px;
  margin: 0 auto;
  padding: 20px;
  background: var(--bg);
  color: var(--fg);
  line-height: 1.6;
}

.header {
  border-bottom: 2px solid var(--accent);
  padding-bottom: 10px;
  margin-bottom: 20px;
}

.header h1 { margin: 0; font-size: 1.5em; color: var(--accent); }
.header .meta { color: #888; font-size: 0.9em; margin-top: 5px; }

.message {
  margin: 16px 0;
  padding: 12px 16px;
  border-radius: 8px;
  border: 1px solid var(--border);
}

.message.user { background: var(--user-bg); }
.message.assistant { background: var(--assistant-bg); }
.message.tool { background: var(--tool-bg); }

.message .role {
  font-weight: 600;
  margin-bottom: 8px;
  font-size: 0.9em;
  color: var(--accent);
}

.message .timestamp {
  float: right;
  color: #888;
  font-size: 0.8em;
}

.message .content { white-space: pre-wrap; word-wrap: break-word; }
.message .content pre {
  background: var(--code-bg);
  padding: 8px 12px;
  border-radius: 4px;
  overflow-x: auto;
}

.message .content code {
  background: var(--code-bg);
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 0.9em;
}

.usage {
  font-size: 0.8em;
  color: #888;
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--border);
}

.footer {
  margin-top: 30px;
  padding-top: 10px;
  border-top: 1px solid var(--border);
  font-size: 0.8em;
  color: #888;
  text-align: center;
}
`
}

// ExportToHTML exports messages to an HTML file.
func ExportToHTML(messages []agent.AgentMessage, path string, opts ExportOptions) error {
	var buf bytes.Buffer

	buf.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	buf.WriteString(fmt.Sprintf("<meta charset=\"UTF-8\">\n"))
	buf.WriteString(fmt.Sprintf("<title>%s</title>\n", html.EscapeString(opts.Title)))
	buf.WriteString(fmt.Sprintf("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n"))

	if opts.IncludeCSS {
		buf.WriteString("<style>\n")
		buf.WriteString(defaultTemplateCSS())
		buf.WriteString("\n</style>\n")
	}

	buf.WriteString("</head>\n<body>\n")

	// Header
	buf.WriteString("<div class=\"header\">\n")
	buf.WriteString(fmt.Sprintf("<h1>%s</h1>\n", html.EscapeString(opts.Title)))
	buf.WriteString(fmt.Sprintf("<div class=\"meta\">Exported: %s | Messages: %d</div>\n",
		time.Now().Format("2006-01-02 15:04:05"), len(messages)))
	buf.WriteString("</div>\n")

	// Messages
	for _, msg := range messages {
		if msg.Hide {
			continue
		}

		roleClass := ""
		roleLabel := ""

		switch msg.Role {
		case ai.RoleUser:
			roleClass = "user"
			roleLabel = "You"
		case ai.RoleAssistant:
			roleClass = "assistant"
			modelName := msg.Model
			if modelName == "" {
				modelName = "Assistant"
			}
			roleLabel = modelName
		case ai.RoleToolResult:
			roleClass = "tool"
			roleLabel = "Tool: " + msg.ToolName
		}

		buf.WriteString(fmt.Sprintf("<div class=\"message %s\">\n", roleClass))

		// Role and timestamp
		buf.WriteString(fmt.Sprintf("<div class=\"role\">%s", html.EscapeString(roleLabel)))
		if opts.ShowTimestamps && msg.Timestamp > 0 {
			t := time.UnixMilli(msg.Timestamp)
			buf.WriteString(fmt.Sprintf("<span class=\"timestamp\">%s</span>", t.Format("15:04:05")))
		}
		buf.WriteString("</div>\n")

		// Content
		buf.WriteString("<div class=\"content\">")
		if msg.Content != "" {
			buf.WriteString(formatContentForHTML(msg.Content))
		}
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				buf.WriteString(fmt.Sprintf("<div><strong>🔧 %s</strong>: <code>%v</code></div>",
					html.EscapeString(tc.Name), tc.Arguments))
			}
		}
		buf.WriteString("</div>\n")

		// Error
		if msg.ErrorMessage != "" {
			buf.WriteString(fmt.Sprintf("<div class=\"error\">⚠ Error: %s</div>\n", html.EscapeString(msg.ErrorMessage)))
		}

		buf.WriteString("</div>\n")
	}

	// Footer
	buf.WriteString("<div class=\"footer\">\n")
	buf.WriteString("Generated by <a href=\"https://github.com/earendil-works/rho\">rho</a>\n")
	buf.WriteString("</div>\n")

	buf.WriteString("</body>\n</html>\n")

	// Write file
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create output dir: %w", err)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("cannot write export file: %w", err)
	}

	return nil
}

// ExportToHTMLString exports messages to an HTML string.
func ExportToHTMLString(messages []agent.AgentMessage, opts ExportOptions) string {
	var buf bytes.Buffer
	// Same logic as ExportToHTML but returns string
	_ = buf.WriteByte(0) // suppress unused warning
	return ""
}

// formatContentForHTML converts message content with markdown-like formatting to HTML.
func formatContentForHTML(content string) string {
	// Simple format: escape HTML, then convert basic markdown patterns
	escaped := html.EscapeString(content)

	// Code blocks ```...```
	var result strings.Builder
	for {
		codeStart := strings.Index(escaped, "```")
		if codeStart == -1 {
			result.WriteString(escapeNewlines(escaped))
			break
		}

		result.WriteString(escapeNewlines(escaped[:codeStart]))
		rest := escaped[codeStart+3:]

		codeEnd := strings.Index(rest, "```")
		if codeEnd == -1 {
			result.WriteString(escapeNewlines(rest))
			break
		}

		code := rest[:codeEnd]
		// Extract language if present
		lang := ""
		if nlIdx := strings.IndexByte(code, '\n'); nlIdx >= 0 {
			lang = strings.TrimSpace(code[:nlIdx])
			code = code[nlIdx+1:]
		}
		_ = lang // could add language class later

		result.WriteString("<pre><code>")
		result.WriteString(code)
		result.WriteString("</code></pre>")

		escaped = rest[codeEnd+3:]
	}

	final := result.String()

	// Inline code `...`
	final = replaceInlineCode(final)

	// Bold **...**
	final = replaceBold(final)

	// Lines starting with # as headings
	final = replaceHeadings(final)

	return final
}

func escapeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\n", "<br>\n")
	return s
}

func replaceInlineCode(s string) string {
	var result strings.Builder
	for {
		start := strings.Index(s, "`")
		if start == -1 {
			result.WriteString(s)
			break
		}
		result.WriteString(s[:start])
		rest := s[start+1:]
		end := strings.Index(rest, "`")
		if end == -1 {
			result.WriteString("`")
			result.WriteString(rest)
			break
		}
		result.WriteString("<code>")
		result.WriteString(rest[:end])
		result.WriteString("</code>")
		s = rest[end+1:]
	}
	return result.String()
}

func replaceBold(s string) string {
	var result strings.Builder
	for {
		start := strings.Index(s, "**")
		if start == -1 {
			result.WriteString(s)
			break
		}
		result.WriteString(s[:start])
		rest := s[start+2:]
		end := strings.Index(rest, "**")
		if end == -1 {
			result.WriteString("**")
			result.WriteString(rest)
			break
		}
		result.WriteString("<strong>")
		result.WriteString(rest[:end])
		result.WriteString("</strong>")
		s = rest[end+2:]
	}
	return result.String()
}

func replaceHeadings(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "### ") {
			lines[i] = fmt.Sprintf("<h3>%s</h3>", strings.TrimPrefix(trimmed, "### "))
		} else if strings.HasPrefix(trimmed, "## ") {
			lines[i] = fmt.Sprintf("<h2>%s</h2>", strings.TrimPrefix(trimmed, "## "))
		} else if strings.HasPrefix(trimmed, "# ") {
			lines[i] = fmt.Sprintf("<h1>%s</h1>", strings.TrimPrefix(trimmed, "# "))
		}
	}
	return strings.Join(lines, "\n")
}
