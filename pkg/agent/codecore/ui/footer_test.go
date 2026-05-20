package codecui

import (
	"strings"
	"testing"
)

func TestFooterGitBranch(t *testing.T) {
	f := NewFooter()
	f.SetGitBranch("main")
	lines := f.Render(80)
	if len(lines) == 0 {
		t.Fatal("expected footer lines")
	}
	if !strings.Contains(lines[0], "main") {
		t.Fatalf("footer should contain 'main': %s", lines[0])
	}
}

func TestFooterModelProvider(t *testing.T) {
	f := NewFooter()
	f.SetModel("claude-sonnet-4", "anthropic")
	lines := f.Render(80)
	if !strings.Contains(lines[0], "anthropic/claude-sonnet-4") {
		t.Fatalf("footer should contain model info: %s", lines[0])
	}
}

func TestFooterSessionName(t *testing.T) {
	f := NewFooter()
	f.SetSessionName("my-session")
	lines := f.Render(80)
	if !strings.Contains(lines[0], "my-session") {
		t.Fatalf("footer should contain session name: %s", lines[0])
	}
}

func TestFooterThinkingLevel(t *testing.T) {
	f := NewFooter()
	f.SetThinkingLevel("high")
	lines := f.Render(80)
	if !strings.Contains(lines[0], "high") {
		t.Fatalf("footer should contain thinking level: %s", lines[0])
	}
}

func TestFooterThinkingOffOmits(t *testing.T) {
	f := NewFooter()
	f.SetThinkingLevel("off")
	lines := f.Render(80)
	// Should not contain any thinking level indicator
	for _, line := range lines {
		if strings.Contains(line, "off") && strings.Contains(line, "think") {
			t.Fatalf("footer should not show thinking level when off: %s", line)
		}
	}
}

func TestFooterTokenCount(t *testing.T) {
	f := NewFooter()
	f.SetTokenCount(150, 8192)
	lines := f.Render(80)
	if !strings.Contains(lines[0], "150") || !strings.Contains(lines[0], "8192") {
		t.Fatalf("footer should contain token info: %s", lines[0])
	}
}

func TestFooterTokenCountWithoutWindow(t *testing.T) {
	f := NewFooter()
	f.SetTokenCount(150, 0)
	lines := f.Render(80)
	if !strings.Contains(lines[0], "tok") {
		t.Fatalf("footer should contain 'tok': %s", lines[0])
	}
}

func TestFooterCost(t *testing.T) {
	f := NewFooter()
	f.SetTotalCost(0.0123)
	lines := f.Render(80)
	if !strings.Contains(lines[0], "$0.0123") {
		t.Fatalf("footer should contain cost: %s", lines[0])
	}
}

func TestFooterAuthStatus(t *testing.T) {
	f := NewFooter()
	f.SetAuthStatus("OAuth", true)
	lines := f.Render(80)
	if !strings.Contains(lines[0], "OAuth") {
		t.Fatalf("footer should contain auth status: %s", lines[0])
	}
}

func TestFooterExtensionStatus(t *testing.T) {
	f := NewFooter()
	f.SetStatus("linter", "linting...")
	lines := f.Render(80)
	if !strings.Contains(lines[0], "linting...") {
		t.Fatalf("footer should contain extension status: %s", lines[0])
	}
}

func TestFooterShowHelp(t *testing.T) {
	f := NewFooter()
	f.SetShowHelp(true)
	lines := f.Render(80)
	if !strings.Contains(lines[0], "for help") {
		t.Fatalf("footer should contain help hint: %s", lines[0])
	}
}

func TestFooterZeroWidth(t *testing.T) {
	f := NewFooter()
	lines := f.Render(0)
	if lines != nil {
		t.Fatal("expected nil for zero width")
	}
}

func TestFooterEmpty(t *testing.T) {
	f := NewFooter()
	lines := f.Render(80)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
}

func TestFooterMultipleExtensions(t *testing.T) {
	f := NewFooter()
	f.SetStatus("ext1", "status1")
	f.SetStatus("ext2", "status2")
	lines := f.Render(80)
	if !strings.Contains(lines[0], "status1") {
		t.Fatalf("footer should contain status1: %s", lines[0])
	}
	if !strings.Contains(lines[0], "status2") {
		t.Fatalf("footer should contain status2: %s", lines[0])
	}
}

func TestFooterClearExtension(t *testing.T) {
	f := NewFooter()
	f.SetStatus("ext1", "visible")
	f.SetStatus("ext1", "") // clear
	lines := f.Render(80)
	if strings.Contains(lines[0], "visible") {
		t.Fatalf("footer should not contain cleared extension status")
	}
}

func TestFooterAllFields(t *testing.T) {
	f := NewFooter()
	f.SetGitBranch("feature-branch")
	f.SetSessionName("work-session")
	f.SetModel("gpt-4o", "openai")
	f.SetThinkingLevel("medium")
	f.SetTokenCount(500, 128000)
	f.SetTotalCost(0.05)
	f.SetAuthStatus("OAuth", true)
	f.SetStatus("search", "indexing...")
	f.SetShowHelp(true)

	lines := f.Render(180)
	combined := strings.Join(lines, " ")
	checks := []string{"feature-branch", "work-session", "openai/gpt-4o", "medium", "500", "128000", "0.05", "OAuth", "indexing...", "for help"}
	for _, c := range checks {
		if !strings.Contains(combined, c) {
			t.Fatalf("footer missing %q: %s", c, combined)
		}
	}
}
