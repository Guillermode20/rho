package agentutils

import (
	"strings"
	"testing"
)

func TestStripHTMLTagsRemovesScriptAndStyleBlocks(t *testing.T) {
	got := StripHTMLTags(`<style>.hidden{display:none}</style><p>Hello<br>world</p><script>alert("x")</script>`)

	if strings.Contains(got, "hidden") || strings.Contains(got, "alert") {
		t.Fatalf("StripHTMLTags kept script/style content: %q", got)
	}
	if got != "Hello\nworld" {
		t.Fatalf("StripHTMLTags = %q, want Hello\\nworld", got)
	}
}
