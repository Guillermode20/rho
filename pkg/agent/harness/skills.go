package harness

import (
	"bufio"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/earendil-works/rho/pkg/agent"
)

const (
	maxNameLength        = 64
	maxDescriptionLength = 1024
)

var frontmatterRe = regexp.MustCompile(`(?s)^---\n(.+?)\n---\n?(.*)$`)

// SkillDiagnosticCode represents the code for a skill diagnostic.
type SkillDiagnosticCode string

const (
	SkillDiagFileInfoFailed SkillDiagnosticCode = "file_info_failed"
	SkillDiagListFailed     SkillDiagnosticCode = "list_failed"
	SkillDiagReadFailed     SkillDiagnosticCode = "read_failed"
	SkillDiagParseFailed    SkillDiagnosticCode = "parse_failed"
	SkillDiagInvalidMeta    SkillDiagnosticCode = "invalid_metadata"
)

// LoadSkills loads skills from one or more directories.
// Traverses directories recursively for SKILL.md and root .md files.
func LoadSkills(env ExecutionEnv, dirs []string) ([]Skill, []SkillDiagnostic) {
	var skills []Skill
	var diagnostics []SkillDiagnostic
	seen := make(map[string]bool)

	for _, dir := range dirs {
		infoResult := env.FileInfo(dir)
		if !infoResult.Ok {
			if infoResult.Error.Code != FileErrNotFound {
				diagnostics = append(diagnostics, SkillDiagnostic{
					Type: "warning", Code: string(SkillDiagFileInfoFailed),
					Message: infoResult.Error.Message, Path: dir,
				})
			}
			continue
		}
		if infoResult.Value.Kind != FileKindDirectory {
			continue
		}

		result := loadSkillsFromDir(env, dir, seen)
		skills = append(skills, result.skills...)
		diagnostics = append(diagnostics, result.diagnostics...)
	}

	return skills, diagnostics
}

func loadSkillsFromDir(env ExecutionEnv, dir string, seen map[string]bool) struct {
	skills      []Skill
	diagnostics []SkillDiagnostic
} {
	var skills []Skill
	var diagnostics []SkillDiagnostic

	entriesResult := env.ListDir(dir)
	if !entriesResult.Ok {
		diagnostics = append(diagnostics, SkillDiagnostic{
			Type: "warning", Code: string(SkillDiagListFailed),
			Message: entriesResult.Error.Message, Path: dir,
		})
		return struct {
			skills      []Skill
			diagnostics []SkillDiagnostic
		}{skills, diagnostics}
	}

	entries := entriesResult.Value

	// First pass: look for SKILL.md
	for _, entry := range entries {
		if entry.Name != "SKILL.md" || entry.Kind != FileKindFile {
			continue
		}
		if seen[entry.Path] {
			continue
		}
		seen[entry.Path] = true

		skill, diag := loadSkillFromFile(env, entry.Path)
		if skill != nil {
			skills = append(skills, *skill)
		}
		if diag != nil {
			diagnostics = append(diagnostics, *diag)
		}

		// After finding SKILL.md, also load direct .md files in the root
		for _, e := range entries {
			if e.Name == "SKILL.md" || e.Kind != FileKindFile {
				continue
			}
			if !strings.HasSuffix(e.Name, ".md") {
				continue
			}
			if seen[e.Path] {
				continue
			}
			seen[e.Path] = true
			skill2, diag2 := loadSkillFromFile(env, e.Path)
			if skill2 != nil {
				skills = append(skills, *skill2)
			}
			if diag2 != nil {
				diagnostics = append(diagnostics, *diag2)
			}
		}

		return struct {
			skills      []Skill
			diagnostics []SkillDiagnostic
		}{skills, diagnostics}
	}

	// No SKILL.md found - check for subdirectories
	for _, entry := range entries {
		if entry.Kind != FileKindDirectory {
			continue
		}
		subResult := loadSkillsFromDir(env, entry.Path, seen)
		skills = append(skills, subResult.skills...)
		diagnostics = append(diagnostics, subResult.diagnostics...)
	}

	return struct {
		skills      []Skill
		diagnostics []SkillDiagnostic
	}{skills, diagnostics}
}

func loadSkillFromFile(env ExecutionEnv, path string) (*Skill, *SkillDiagnostic) {
	readResult := env.ReadTextFile(path)
	if !readResult.Ok {
		return nil, &SkillDiagnostic{
			Type: "warning", Code: string(SkillDiagReadFailed),
			Message: readResult.Error.Message, Path: path,
		}
	}

	return parseSkillContent(readResult.Value, path)
}

func parseSkillContent(content, filePath string) (*Skill, *SkillDiagnostic) {
	matches := frontmatterRe.FindStringSubmatch(content)
	if matches == nil {
		// No frontmatter - use filename as skill name
		name := strings.TrimSuffix(filepath.Base(filePath), ".md")
		return &Skill{
			Name:        name,
			Description: "No description provided.",
			Content:     content,
			FilePath:    filePath,
		}, nil
	}

	frontmatterRaw := matches[1]
	body := matches[2]

	// Parse simple YAML-like frontmatter
	fm := parseSimpleFrontmatter(frontmatterRaw)

	name := fm["name"]
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(filePath), ".md")
	}
	if len(name) > maxNameLength {
		name = name[:maxNameLength]
	}

	description := fm["description"]
	if description == "" {
		description = "No description provided."
	}
	if len(description) > maxDescriptionLength {
		description = description[:maxDescriptionLength]
	}

	skill := &Skill{
		Name:        name,
		Description: description,
		Content:     strings.TrimSpace(body),
		FilePath:    filePath,
	}

	if fm["disable-model-invocation"] == "true" {
		skill.DisableModelInvocation = true
	}

	return skill, nil
}

func parseSimpleFrontmatter(raw string) map[string]string {
	result := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result
}

// FormatSkillsForPrompt formats skills for inclusion in the system prompt.
func FormatSkillsForPrompt(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n## Available Skills\n")
	b.WriteString("You can invoke skills using <skill> tags:\n\n")
	for _, s := range skills {
		if s.DisableModelInvocation {
			continue
		}
		b.WriteString("- " + s.Name + ": " + s.Description + "\n")
	}
	return b.String()
}

// FindSkill finds a skill by name (case-insensitive).
func FindSkill(skills []Skill, name string) *Skill {
	nameLower := strings.ToLower(name)
	for _, s := range skills {
		if strings.ToLower(s.Name) == nameLower {
			return &s
		}
	}
	return nil
}

// SkillNames returns the names of all visible skills.
func SkillNames(skills []Skill) []string {
	var names []string
	for _, s := range skills {
		if !s.DisableModelInvocation {
			names = append(names, s.Name)
		}
	}
	return names
}

// Ensure compatibility
var _ = agent.AgentMessage{}
