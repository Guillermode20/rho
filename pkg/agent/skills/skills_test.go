package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSkillFileWithFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte("---\nname: My Skill\ndescription: A test\n---\nBody text"), 0644)

	skill, err := parseSkillFile(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if skill.Name != "My Skill" {
		t.Errorf("expected 'My Skill', got '%s'", skill.Name)
	}
}

func TestParseSkillFileNoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "simple.md")
	os.WriteFile(path, []byte("Just text"), 0644)

	skill, err := parseSkillFile(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if skill.Name != "simple" {
		t.Errorf("expected 'simple', got '%s'", skill.Name)
	}
}

func TestLoadSkillsFromDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("---\nname: A\n---\nA"), 0644)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("---\nname: B\n---\nB"), 0644)
	r := LoadSkillsFromDir(dir)
	if len(r.Loaded) != 2 {
		t.Errorf("expected 2, got %d", len(r.Loaded))
	}
}

func TestFormatSkillsForPrompt(t *testing.T) {
	r := FormatSkillsForPrompt([]Skill{{Name: "X", Description: "desc", Content: "content"}})
	if !strings.Contains(r, "X") {
		t.Error("expected 'X'")
	}
}

func TestFilterSkillsByGlob(t *testing.T) {
	skills := []Skill{
		{Name: "Go", Glob: "*.go"},
		{Name: "All", Glob: ""},
	}
	r := FilterSkillsByGlob(skills, "main.go")
	if len(r) != 2 {
		t.Errorf("expected 2, got %d", len(r))
	}
}

func TestParseFrontmatter(t *testing.T) {
	fm, body := ParseFrontmatter("---\nname: x\n---\nbody")
	if fm["name"] != "x" || body != "body" {
		t.Errorf("fm=%v body=%s", fm, body)
	}
}

func TestParseFrontmatterNone(t *testing.T) {
	fm, body := ParseFrontmatter("text")
	if len(fm) != 0 || body != "text" {
		t.Error("expected no frontmatter")
	}
}

func TestStripFrontmatter(t *testing.T) {
	r := StripFrontmatter("---\nn: v\n---\nbody")
	if r != "body" {
		t.Errorf("expected 'body', got '%s'", r)
	}
}

func TestAllTags(t *testing.T) {
	r := AllTags([]Skill{
		{Tags: []string{"a", "b"}},
		{Tags: []string{"b", "c"}},
	})
	if len(r) != 3 {
		t.Errorf("expected 3, got %d", len(r))
	}
}

func TestLoadSkills(t *testing.T) {
	d1, d2 := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(d1, "a.md"), []byte("---\nname: A\n---\nA"), 0644)
	os.WriteFile(filepath.Join(d2, "b.md"), []byte("---\nname: B\n---\nB"), 0644)
	r := LoadSkills([]string{d1, d2})
	if len(r.Loaded) != 2 {
		t.Errorf("expected 2, got %d", len(r.Loaded))
	}
}
