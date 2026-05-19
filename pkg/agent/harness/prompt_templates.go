package harness

import (
	"strings"
	"text/template"
)

// TemplateEngine handles prompt template parsing and interpolation.
type TemplateEngine struct {
	templates map[string]*template.Template
	funcs     template.FuncMap
}

// NewTemplateEngine creates a new template engine.
func NewTemplateEngine() *TemplateEngine {
	return &TemplateEngine{
		templates: make(map[string]*template.Template),
		funcs: template.FuncMap{
			"upper": strings.ToUpper,
			"lower": strings.ToLower,
			"trim":  strings.TrimSpace,
			"join":  strings.Join,
		},
	}
}

// Register adds a template to the engine.
func (e *TemplateEngine) Register(name, content string) error {
	tmpl, err := template.New(name).Funcs(e.funcs).Parse(content)
	if err != nil {
		return err
	}
	e.templates[name] = tmpl
	return nil
}

// Execute renders a template with the given data.
func (e *TemplateEngine) Execute(name string, data interface{}) (string, error) {
	tmpl, ok := e.templates[name]
	if !ok {
		return "", &AgentHarnessError{
			Code:    HarnessErrInvalidArg,
			Message: "template not found: " + name,
		}
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// FormatPromptTemplate formats a prompt template with variable substitution.
// Supports {{.Variable}} syntax.
func FormatPromptTemplate(content string, vars map[string]string) string {
	result := content
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{{."+k+"}}", v)
	}
	return result
}

// BuildSystemPromptFromTemplate builds a system prompt using a registered template.
func BuildSystemPromptFromTemplate(engine *TemplateEngine, templateName string, data map[string]interface{}) (string, error) {
	return engine.Execute(templateName, data)
}

// DefaultSystemPromptTemplate returns the default system prompt template.
func DefaultSystemPromptTemplate() string {
	return `You are rho, a coding agent.

{{if .systemPrompt}}{{.systemPrompt}}
{{end}}
{{if .skills}}## Available Skills
{{range .skills}}- {{.Name}}: {{.Description}}
{{end}}{{end}}
{{if .tools}}## Available Tools
{{range .tools}}- {{.Name}}: {{.Description}}
{{end}}{{end}}
## Guidelines
- Read files before editing them to understand context.
- Use Bash for shell commands. Set timeout for long-running commands.
- Use Edit for exact text replacements in existing files.
- Use Write for new files or complete overwrites.
- Use Grep to search file contents.
- Use Find to locate files by name.
- Think step by step before making changes.
{{if .additionalGuidelines}}
{{.additionalGuidelines}}
{{end}}`
}

// FormatSkillInvocation formats a skill for invocation in the system prompt.
func FormatSkillInvocation(skill Skill, additionalInstructions string) string {
	skillBlock := FormatSkillXMLBlock(skill)
	if additionalInstructions != "" {
		return skillBlock + "\n\n" + additionalInstructions
	}
	return skillBlock
}

// FormatSkillXMLBlock formats a skill as an XML block.
func FormatSkillXMLBlock(skill Skill) string {
	return "<skill name=\"" + skill.Name + "\" location=\"" + skill.FilePath + "\">\n" +
		skill.Content +
		"\n</skill>"
}

// FormatSkillsForSystemPrompt formats skills for inclusion in the system prompt.
func FormatSkillsForSystemPrompt(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n## Available Skills\n")
	b.WriteString("You have access to the following skills. Use <skill> tags to invoke them:\n\n")
	for _, s := range skills {
		if s.DisableModelInvocation {
			continue
		}
		b.WriteString("- **" + s.Name + "**: " + s.Description + "\n")
	}
	return b.String()
}

// FormatPromptTemplateInvocation formats a prompt template for explicit invocation.
func FormatPromptTemplateInvocation(tmpl PromptTemplate, args map[string]string) string {
	return FormatPromptTemplate(tmpl.Content, args)
}
