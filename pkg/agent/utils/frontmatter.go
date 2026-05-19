package agentutils

import (
	"strings"
)

// Frontmatter contains parsed YAML frontmatter and the remaining content.
type Frontmatter struct {
	Raw     string            // Raw YAML frontmatter text (between the --- delimiters)
	Data    map[string]string // Parsed key-value pairs (simple parser)
	Content string            // Content after the frontmatter
	Has     bool              // Whether frontmatter was found
}

// ParseFrontmatter parses YAML frontmatter from markdown content.
// Frontmatter is delimited by --- or +++ lines at the start of the file.
func ParseFrontmatter(content string) Frontmatter {
	result := Frontmatter{Data: make(map[string]string), Content: content}

	trimmed := strings.TrimLeft(content, " \t\r\n")
	if trimmed == "" {
		return result
	}

	// Check if file starts with --- or +++
	if !strings.HasPrefix(trimmed, "---") && !strings.HasPrefix(trimmed, "+++") {
		return result
	}

	delimiter := trimmed[:3]
	rest := trimmed[3:]

	// Skip optional newline after delimiter
	rest = strings.TrimLeft(rest, " \t\r\n")

	// Find closing delimiter
	endIdx := strings.Index(rest, "\n"+delimiter)
	if endIdx < 0 {
		// Try at start of line
		endIdx = strings.Index(rest, delimiter)
		if endIdx < 0 {
			return result
		}
	}

	frontRaw := strings.TrimSpace(rest[:endIdx])
	afterDelim := rest[endIdx+len(delimiter):]
	contentAfter := strings.TrimLeft(afterDelim, " \t\r\n")

	result.Raw = frontRaw
	result.Content = contentAfter
	result.Has = true

	// Parse simple key: value pairs
	for _, line := range strings.Split(frontRaw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])
		// Strip quotes
		value = strings.Trim(value, "\"'")
		result.Data[key] = value
	}

	return result
}

// StripFrontmatter removes frontmatter from content and returns the remaining text.
func StripFrontmatter(content string) string {
	fm := ParseFrontmatter(content)
	if fm.Has {
		return fm.Content
	}
	return content
}

// ExtractFrontmatterValue extracts a single value from frontmatter by key.
func ExtractFrontmatterValue(content, key string) (string, bool) {
	fm := ParseFrontmatter(content)
	val, ok := fm.Data[key]
	return val, ok
}

// HasFrontmatterField checks if a specific frontmatter field exists.
func HasFrontmatterField(content, key string) bool {
	_, ok := ExtractFrontmatterValue(content, key)
	return ok
}

// BuildFrontmatter builds a YAML frontmatter string from key-value pairs.
func BuildFrontmatter(data map[string]string) string {
	if len(data) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("---\n")
	for k, v := range data {
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(v)
		b.WriteString("\n")
	}
	b.WriteString("---\n")
	return b.String()
}
