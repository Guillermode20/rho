// Package resources loads project context files and resources.
package resources

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResourceDescriptor describes a discovered resource.
type ResourceDescriptor struct {
	Path     string `json:"path"`
	Type     string `json:"type"` // "skill", "prompt", "theme", "context"
	Source   string `json:"source"`
	Priority int    `json:"priority"`
}

// ResourceLoader discovers and loads resources.
type ResourceLoader struct {
	projectDir string
	resources  []ResourceDescriptor
}

// NewResourceLoader creates a resource loader for a project.
func NewResourceLoader(projectDir string) *ResourceLoader {
	return &ResourceLoader{
		projectDir: projectDir,
	}
}

// Discover finds all resources in the project.
func (rl *ResourceLoader) Discover() ([]ResourceDescriptor, error) {
	var resources []ResourceDescriptor

	// Project context files (AGENTS.md, CLAUDE.md)
	contextFiles := []string{"AGENTS.md", "AGENTS.MD", "CLAUDE.md", "CLAUDE.MD", ".rho/context.md", ".github/AGENTS.md"}
	for _, f := range contextFiles {
		path := filepath.Join(rl.projectDir, f)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			resources = append(resources, ResourceDescriptor{
				Path:     path,
				Type:     "context",
				Source:   "project",
				Priority: 100,
			})
		}
	}

	// .rho/skills/ directory
	skillsDir := filepath.Join(rl.projectDir, ".rho", "skills")
	if info, err := os.Stat(skillsDir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(skillsDir)
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
				resources = append(resources, ResourceDescriptor{
					Path:     filepath.Join(skillsDir, e.Name()),
					Type:     "skill",
					Source:   "project",
					Priority: 80,
				})
			}
		}
	}

	// .rho/prompts/ directory
	promptsDir := filepath.Join(rl.projectDir, ".rho", "prompts")
	if info, err := os.Stat(promptsDir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(promptsDir)
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
				resources = append(resources, ResourceDescriptor{
					Path:     filepath.Join(promptsDir, e.Name()),
					Type:     "prompt",
					Source:   "project",
					Priority: 70,
				})
			}
		}
	}

	// .rho/themes/ directory
	themesDir := filepath.Join(rl.projectDir, ".rho", "themes")
	if info, err := os.Stat(themesDir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(themesDir)
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
				resources = append(resources, ResourceDescriptor{
					Path:     filepath.Join(themesDir, e.Name()),
					Type:     "theme",
					Source:   "project",
					Priority: 60,
				})
			}
		}
	}

	sort.Slice(resources, func(i, j int) bool {
		return resources[i].Priority > resources[j].Priority
	})

	rl.resources = resources
	return resources, nil
}

// ReadProjectContext reads and combines project context files.
func ReadProjectContext(projectDir string) (string, error) {
	rl := NewResourceLoader(projectDir)
	resources, err := rl.Discover()
	if err != nil {
		return "", err
	}

	var parts []string
	for _, r := range resources {
		if r.Type != "context" {
			continue
		}
		data, err := os.ReadFile(r.Path)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content != "" {
			parts = append(parts, fmt.Sprintf("## From %s\n\n%s", filepath.Base(r.Path), content))
		}
	}

	return strings.Join(parts, "\n\n"), nil
}

// LoadProjectContextFiles is a convenience function.
func LoadProjectContextFiles(projectDir string) ([]ResourceDescriptor, string, error) {
	rl := NewResourceLoader(projectDir)
	resources, err := rl.Discover()
	if err != nil {
		return resources, "", err
	}
	context, _ := ReadProjectContext(projectDir)
	return resources, context, nil
}
