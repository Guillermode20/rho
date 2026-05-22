package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Skill represents a loaded skill with parsed frontmatter.
type Skill struct {
	Name                   string   `json:"name"`
	Description            string   `json:"description"`
	Content                string   `json:"content"`
	SourceFile             string   `json:"sourceFile,omitempty"`
	Tags                   []string `json:"tags,omitempty"`
	Glob                   string   `json:"glob,omitempty"`
	DisableModelInvocation bool     `json:"disableModelInvocation,omitempty"`
}

// LoadSkillsResult summarizes a skill loading operation.
type LoadSkillsResult struct {
	Loaded  []Skill  `json:"loaded"`
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

// LoadSkillsFromDir recursively loads skills from a directory.
// Scans for .md and .mdx files with YAML frontmatter.
func LoadSkillsFromDir(dir string) *LoadSkillsResult {
	result := &LoadSkillsResult{}

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			if info.Name() == "node_modules" || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(info.Name()))
		if ext != ".md" && ext != ".mdx" {
			return nil
		}

		skill, err := parseSkillFile(path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", info.Name(), err))
			result.Skipped++
			return nil
		}

		if skill != nil {
			result.Loaded = append(result.Loaded, *skill)
		}
		return nil
	})

	return result
}

var skillNameRegex = regexp.MustCompile(`^[a-z0-9-]+$`)

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
	disableModel := false
	body := content

	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content[3:], "---", 2)
		if len(parts) == 2 {
			fm := strings.TrimSpace(parts[0])
			body = strings.TrimSpace(parts[1])

			// Simple frontmatter parser
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
				case strings.HasPrefix(line, "disable-model-invocation:"):
					val := strings.TrimSpace(strings.TrimPrefix(line, "disable-model-invocation:"))
					if val == "true" {
						disableModel = true
					}
				}
			}
		}
	}

	if len(name) > 64 {
		return nil, fmt.Errorf("name exceeds 64 characters")
	}
	if !skillNameRegex.MatchString(name) {
		return nil, fmt.Errorf("name must match ^[a-z0-9-]+$")
	}
	if desc == "" {
		return nil, fmt.Errorf("description is required")
	}
	if len(desc) > 1024 {
		return nil, fmt.Errorf("description exceeds 1024 characters")
	}

	return &Skill{
		Name:                   name,
		Description:            desc,
		Content:                body,
		SourceFile:             path,
		Tags:                   tags,
		Glob:                   glob,
		DisableModelInvocation: disableModel,
	}, nil
}

// FormatSkillsForPrompt formats loaded skills into an XML index for lazy loading.
func FormatSkillsForPrompt(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var parts []string
	parts = append(parts, "<available_skills>")

	for _, skill := range skills {
		if skill.DisableModelInvocation {
			continue
		}
		parts = append(parts, "  <skill>")
		parts = append(parts, fmt.Sprintf("    <name>%s</name>", skill.Name))
		parts = append(parts, fmt.Sprintf("    <description>%s</description>", skill.Description))
		parts = append(parts, fmt.Sprintf("    <location>%s</location>", skill.SourceFile))
		parts = append(parts, "  </skill>")
	}

	parts = append(parts, "</available_skills>")
	parts = append(parts, "To view the full instructions of a skill, use your `Read` tool on its `<location>` path.")

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
