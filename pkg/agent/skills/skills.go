// Package skills manages skill files — reusable markdown snippets with YAML frontmatter
// that can be injected into the system prompt.
package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skill represents a loaded skill with parsed frontmatter.
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	SourceFile  string `json:"sourceFile,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Glob        string `json:"glob,omitempty"` // File pattern this skill applies to
}

// SkillFrontmatter is the parsed YAML frontmatter of a skill file.
type SkillFrontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags,omitempty"`
	Glob        string   `yaml:"glob,omitempty"`
}

// LoadSkillsResult summarizes a skill loading operation.
type LoadSkillsResult struct {
	Loaded  []Skill `json:"loaded"`
	Errors  []string `json:"errors"`
	Skipped int      `json:"skipped"`
}

// LoadSkills loads skills from multiple directories.
func LoadSkills(dirs []string) *LoadSkillsResult {
	result := &LoadSkillsResult{}

	for _, dir := range dirs {
		r := LoadSkillsFromDir(dir)
		result.Loaded = append(result.Loaded, r.Loaded...)
		result.Errors = append(result.Errors, r.Errors...)
		result.Skipped += r.Skipped
	}

	return result
}

// LoadSkillsFromDir loads skills from a single directory.
// Scans for .md and .mdx files with YAML frontmatter.
func LoadSkillsFromDir(dir string) *LoadSkillsResult {
	result := &LoadSkillsResult{}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			result.Errors = append(result.Errors, fmt.Sprintf("cannot read %s: %v", dir, err))
		}
		return result
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".md" && ext != ".mdx" {
			continue
		}

		skill, err := parseSkillFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", entry.Name(), err))
			result.Skipped++
			continue
		}

		if skill != nil {
			result.Loaded = append(result.Loaded, *skill)
		}
	}

	return result
}

// parseSkillFile parses a single skill markdown file with YAML frontmatter.
func parseSkillFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)

	// Parse frontmatter (between --- delimiters)
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	desc := ""
	var tags []string
	glob := ""
	body := content

	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content[3:], "---", 2)
		if len(parts) == 2 {
			fm := strings.TrimSpace(parts[0])
			body = strings.TrimSpace(parts[1])

			// Very simple frontmatter parser
			lines := strings.Split(fm, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				switch {
				case strings.HasPrefix(line, "name:"):
					name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
					name = strings.Trim(name, "\"'")
				case strings.HasPrefix(line, "description:"):
					desc = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
					desc = strings.Trim(desc, "\"'")
				case strings.HasPrefix(line, "tags:"):
					tagList := strings.TrimSpace(strings.TrimPrefix(line, "tags:"))
					tagList = strings.Trim(tagList, "[]")
					for _, t := range strings.Split(tagList, ",") {
						t = strings.TrimSpace(t)
						t = strings.Trim(t, "\"'")
						if t != "" {
							tags = append(tags, t)
						}
					}
				case strings.HasPrefix(line, "glob:"):
					glob = strings.TrimSpace(strings.TrimPrefix(line, "glob:"))
					glob = strings.Trim(glob, "\"'")
				}
			}
		}
	}

	return &Skill{
		Name:        name,
		Description: desc,
		Content:     body,
		SourceFile:  path,
		Tags:        tags,
		Glob:        glob,
	}, nil
}

// FormatSkillsForPrompt formats loaded skills for inclusion in the system prompt.
func FormatSkillsForPrompt(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var parts []string
	parts = append(parts, "--- Skills ---")

	for _, skill := range skills {
		if skill.Description != "" {
			parts = append(parts, fmt.Sprintf("## %s", skill.Name))
			parts = append(parts, "")
			parts = append(parts, skill.Description)
			parts = append(parts, "")
		}
		parts = append(parts, skill.Content)
		parts = append(parts, "")
	}

	return strings.Join(parts, "\n")
}

// FilterSkillsByGlob filters skills by glob pattern.
func FilterSkillsByGlob(skills []Skill, filePath string) []Skill {
	var result []Skill
	for _, skill := range skills {
		if skill.Glob == "" {
			result = append(result, skill)
			continue
		}
		if ok, _ := filepath.Match(skill.Glob, filepath.Base(filePath)); ok {
			result = append(result, skill)
			continue
		}
		if ok, _ := filepath.Match(skill.Glob, filePath); ok {
			result = append(result, skill)
		}
	}
	return result
}

// ParseFrontmatter extracts frontmatter from a markdown string.
func ParseFrontmatter(content string) (frontmatter map[string]string, body string) {
	fm := make(map[string]string)

	if !strings.HasPrefix(content, "---") {
		return fm, content
	}

	parts := strings.SplitN(content[3:], "---", 2)
	if len(parts) != 2 {
		return fm, content
	}

	fmText := strings.TrimSpace(parts[0])
	body = strings.TrimSpace(parts[1])

	for _, line := range strings.Split(fmText, "\n") {
		line = strings.TrimSpace(line)
		colonIdx := strings.Index(line, ":")
		if colonIdx > 0 {
			key := strings.TrimSpace(line[:colonIdx])
			value := strings.TrimSpace(line[colonIdx+1:])
			value = strings.Trim(value, "\"'")
			fm[key] = value
		}
	}

	return fm, body
}

// StripFrontmatter removes YAML frontmatter from markdown content.
func StripFrontmatter(content string) string {
	_, body := ParseFrontmatter(content)
	return body
}

// AllTags returns all unique tags across skills.
func AllTags(skills []Skill) []string {
	tagSet := make(map[string]bool)
	for _, s := range skills {
		for _, t := range s.Tags {
			tagSet[t] = true
		}
	}
	var tags []string
	for t := range tagSet {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags
}
