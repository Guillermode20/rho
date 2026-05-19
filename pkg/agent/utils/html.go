package agentutils

import (
	"regexp"
	"strings"
)

var (
	htmlTagRe     = regexp.MustCompile(`<[^>]*>`)
	htmlEntityRe  = regexp.MustCompile(`&[a-zA-Z]+;|&#[0-9]+;|&#x[0-9a-fA-F]+;`)
	htmlBreakRe   = regexp.MustCompile(`<br\s*/?>`)
	htmlParaRe    = regexp.MustCompile(`</p>`)
	multiSpaceRe  = regexp.MustCompile(`\s{2,}`)
	htmlCommentRe = regexp.MustCompile(`<!--.*?-->`)
	scriptStyleRe = regexp.MustCompile(`<(script|style)[^>]*>.*?</\1>`)
)

// StripHTMLTags removes all HTML tags from a string, preserving text content.
func StripHTMLTags(s string) string {
	s = scriptStyleRe.ReplaceAllString(s, "")
	s = htmlCommentRe.ReplaceAllString(s, "")
	s = htmlBreakRe.ReplaceAllString(s, "\n")
	s = htmlParaRe.ReplaceAllString(s, "\n\n")
	s = htmlTagRe.ReplaceAllString(s, "")
	s = htmlEntityRe.ReplaceAllStringFunc(s, decodeHTMLEntity)
	s = multiSpaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// EscapeHTML escapes HTML special characters.
func EscapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// UnescapeHTML unescapes HTML entities.
func UnescapeHTML(s string) string {
	return htmlEntityRe.ReplaceAllStringFunc(s, decodeHTMLEntity)
}

// WrapHTML wraps content in a basic HTML document structure.
func WrapHTML(title, body string) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	b.WriteString("<meta charset=\"UTF-8\">\n")
	if title != "" {
		b.WriteString("<title>")
		b.WriteString(EscapeHTML(title))
		b.WriteString("</title>\n")
	}
	b.WriteString("</head>\n<body>\n")
	b.WriteString(body)
	b.WriteString("\n</body>\n</html>")
	return b.String()
}

// ExtractTextFromHTML extracts the visible text content from HTML.
func ExtractTextFromHTML(s string) string {
	return StripHTMLTags(s)
}

// HTMLToMarkdown converts simple HTML to markdown.
func HTMLToMarkdown(s string) string {
	// Convert common HTML to markdown
	result := s

	// Headers
	result = regexp.MustCompile(`<h1[^>]*>(.*?)</h1>`).ReplaceAllString(result, "# $1\n\n")
	result = regexp.MustCompile(`<h2[^>]*>(.*?)</h2>`).ReplaceAllString(result, "## $1\n\n")
	result = regexp.MustCompile(`<h3[^>]*>(.*?)</h3>`).ReplaceAllString(result, "### $1\n\n")

	// Bold/italic
	result = regexp.MustCompile(`<strong>(.*?)</strong>`).ReplaceAllString(result, "**$1**")
	result = regexp.MustCompile(`<em>(.*?)</em>`).ReplaceAllString(result, "*$1*")

	// Links
	result = regexp.MustCompile(`<a\s+[^>]*href="([^"]*)"[^>]*>(.*?)</a>`).ReplaceAllString(result, "[$2]($1)")

	// Images
	result = regexp.MustCompile(`<img\s+[^>]*src="([^"]*)"[^>]*alt="([^"]*)"[^>]*/>`).ReplaceAllString(result, "![$2]($1)")
	result = regexp.MustCompile(`<img\s+[^>]*src="([^"]*)"[^>]*>`).ReplaceAllString(result, "![]($1)")

	// Code
	result = regexp.MustCompile(`<code>(.*?)</code>`).ReplaceAllString(result, "`$1`")
	result = regexp.MustCompile(`<pre>(.*?)</pre>`).ReplaceAllString(result, "```\n$1\n```")

	// Lists
	result = regexp.MustCompile(`<li>(.*?)</li>`).ReplaceAllString(result, "- $1\n")

	// Horizontal rules
	result = regexp.MustCompile(`<hr\s*/?>`).ReplaceAllString(result, "---\n\n")

	// Blockquotes
	result = regexp.MustCompile(`<blockquote>(.*?)</blockquote>`).ReplaceAllString(result, "> $1\n\n")

	// Clean remaining tags
	result = StripHTMLTags(result)

	return strings.TrimSpace(result)
}

func decodeHTMLEntity(entity string) string {
	entityMap := map[string]string{
		"&amp;": "&", "&lt;": "<", "&gt;": ">",
		"&quot;": "\"", "&#39;": "'", "&#x27;": "'",
		"&nbsp;": " ", "&ndash;": "–", "&mdash;": "—",
		"&lsquo;": "'", "&rsquo;": "'", "&ldquo;": "\"", "&rdquo;": "\"",
		"&hellip;": "...", "&copy;": "©", "&reg;": "®",
		"&euro;": "€", "&pound;": "£", "&yen;": "¥",
		"&bull;": "•", "&middot;": "·", "&deg;": "°",
		"&laquo;": "«", "&raquo;": "»", "&trade;": "™",
	}
	if decoded, ok := entityMap[entity]; ok {
		return decoded
	}
	return entity
}
