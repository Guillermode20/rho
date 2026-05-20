package extensions

import (
	"testing"
)

func TestParseTOML(t *testing.T) {
	content := `
id = "test-id"
name = "test-name"
version = "1.0.0"
description = "test-desc"

[entry]
command = "cmd -arg"

[capabilities]
tools = true
skills = false
prompts = true

[[skills]]
id = "s1"
name = "sk1"
description = "d1"

[[tools]]
id = "t1"
description = "d2"
`
	manifest, err := ParseTOML(content)
	if err != nil {
		t.Fatalf("unexpected error parsing TOML: %v", err)
	}

	if manifest.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got %q", manifest.ID)
	}
	if manifest.Name != "test-name" {
		t.Errorf("expected Name 'test-name', got %q", manifest.Name)
	}
	if manifest.Entry.Command != "cmd -arg" {
		t.Errorf("expected command 'cmd -arg', got %q", manifest.Entry.Command)
	}
	if !manifest.Capabilities.Tools {
		t.Errorf("expected Capabilities.Tools to be true")
	}
	if manifest.Capabilities.Skills {
		t.Errorf("expected Capabilities.Skills to be false")
	}
	if len(manifest.Skills) != 1 || manifest.Skills[0].ID != "s1" {
		t.Errorf("expected 1 skill with ID 's1', got %v", manifest.Skills)
	}
	if len(manifest.Tools) != 1 || manifest.Tools[0].ID != "t1" {
		t.Errorf("expected 1 tool with ID 't1', got %v", manifest.Tools)
	}
}
