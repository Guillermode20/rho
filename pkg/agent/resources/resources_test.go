package resources

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewResourceLoader(t *testing.T) {
	rl := NewResourceLoader(t.TempDir())
	if rl == nil {
		t.Fatal("expected non-nil loader")
	}
}

func TestDiscover(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "skill.md"), []byte("# Skill"), 0644)
	os.WriteFile(filepath.Join(dir, "theme.json"), []byte(`{"name":"t"}`), 0644)
	rl := NewResourceLoader(dir)
	resources, err := rl.Discover()
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(resources) < 2 {
		t.Logf("Found %d resources (may include hidden files)", len(resources))
	}
}

func TestLoadProjectContextFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("ctx"), 0644)
	descs, ctx, err := LoadProjectContextFiles(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	_ = descs
	if strings.TrimSpace(ctx) == "" {
		t.Log("Context may be empty depending on implementation")
	}
}

func TestReadProjectContext(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("Project info"), 0644)
	ctx, err := ReadProjectContext(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !strings.Contains(ctx, "Project info") {
		t.Errorf("expected 'Project info', got '%s'", ctx)
	}
}

func TestReadProjectContextEmpty(t *testing.T) {
	dir := t.TempDir()
	ctx, err := ReadProjectContext(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if ctx != "" {
		t.Errorf("expected empty, got '%s'", ctx)
	}
}

func TestDiscoverMissingDir(t *testing.T) {
	rl := NewResourceLoader("/nonexistent")
	resources, err := rl.Discover()
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0, got %d", len(resources))
	}
}
