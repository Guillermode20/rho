package codecore

import (
	"os"
	"path/filepath"
	"strings"
)

type ResolvedResource struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Kind     string `json:"kind"` // "docs", "skill", "prompt", "theme"
	Priority int    `json:"priority"`
}

type ResolvedPaths struct {
	SkillPaths  []string
	PromptPaths []string
	ThemePaths  []string
}

type ResourceLoader struct{ cwd string }

func NewResourceLoader(cwd string) *ResourceLoader { return &ResourceLoader{cwd: cwd} }

func (rl *ResourceLoader) LoadProjectContextFiles() ([]ResolvedResource, error) {
	var resources []ResolvedResource
	priority := 10
	for _, name := range []string{"AGENTS.md", "CLAUDE.md", ".rho/instructions.md", ".rho/context.md", ".rho/project.md"} {
		path := filepath.Join(rl.cwd, name)
		if data, err := os.ReadFile(path); err == nil {
			resources = append(resources, ResolvedResource{Path: path, Content: string(data), Kind: "docs", Priority: priority})
			priority++
		}
	}
	return resources, nil
}

func (rl *ResourceLoader) LoadSkillFiles(dir string) ([]ResolvedResource, error) {
	var resources []ResolvedResource
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return resources, nil
	}
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			if data, err := os.ReadFile(path); err == nil {
				resources = append(resources, ResolvedResource{Path: path, Content: string(data), Kind: "skill"})
			}
		}
		return nil
	})
	return resources, nil
}

func (rl *ResourceLoader) LoadPromptFiles(dir string) ([]ResolvedResource, error) {
	var resources []ResolvedResource
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return resources, nil
	}
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			ext := filepath.Ext(info.Name())
			if ext == ".md" || ext == ".txt" || ext == ".prompt" {
				if data, err := os.ReadFile(path); err == nil {
					resources = append(resources, ResolvedResource{Path: path, Content: string(data), Kind: "prompt"})
				}
			}
		}
		return nil
	})
	return resources, nil
}

func (rl *ResourceLoader) LoadThemeFiles(dir string) ([]ResolvedResource, error) {
	var resources []ResolvedResource
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return resources, nil
	}
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".json") {
			if data, err := os.ReadFile(path); err == nil {
				resources = append(resources, ResolvedResource{Path: path, Content: string(data), Kind: "theme"})
			}
		}
		return nil
	})
	return resources, nil
}

func (rl *ResourceLoader) ResolveResources() (*ResolvedPaths, error) {
	paths := &ResolvedPaths{}
	home, _ := os.UserHomeDir()
	for _, dir := range []string{
		filepath.Join(rl.cwd, ".rho", "skills"),
		filepath.Join(rl.cwd, ".rho", "prompts"),
		filepath.Join(rl.cwd, ".rho", "themes"),
		filepath.Join(home, ".rho", "skills"),
		filepath.Join(home, ".rho", "prompts"),
		filepath.Join(home, ".rho", "themes"),
	} {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			name := filepath.Base(dir)
			switch name {
			case "skills":
				paths.SkillPaths = append(paths.SkillPaths, dir)
			case "prompts":
				paths.PromptPaths = append(paths.PromptPaths, dir)
			case "themes":
				paths.ThemePaths = append(paths.ThemePaths, dir)
			}
		}
	}
	return paths, nil
}

func LoadProjectContextFiles(cwd string) ([]ResolvedResource, error) {
	return NewResourceLoader(cwd).LoadProjectContextFiles()
}
