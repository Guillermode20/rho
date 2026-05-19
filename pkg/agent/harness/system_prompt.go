package harness

import (
	"fmt"
	"strings"

	"github.com/earendil-works/rho/pkg/agent"
)

// SystemPromptSection defines a section in the system prompt.
type SystemPromptSection struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Order   int    `json:"order"`
	Enabled bool   `json:"enabled"`
}

// SystemPromptBuilder builds structured system prompts.
type SystemPromptBuilder struct {
	sections []SystemPromptSection
}

// NewSystemPromptBuilder creates a new system prompt builder.
func NewSystemPromptBuilder() *SystemPromptBuilder {
	return &SystemPromptBuilder{}
}

// AddSection adds a section to the system prompt.
func (b *SystemPromptBuilder) AddSection(name, content string, order int) {
	b.sections = append(b.sections, SystemPromptSection{
		Name: name, Content: content, Order: order, Enabled: true,
	})
}

// RemoveSection removes a section by name.
func (b *SystemPromptBuilder) RemoveSection(name string) {
	for i, s := range b.sections {
		if s.Name == name {
			b.sections = append(b.sections[:i], b.sections[i+1:]...)
			return
		}
	}
}

// EnableSection enables or disables a section.
func (b *SystemPromptBuilder) EnableSection(name string, enabled bool) {
	for i, s := range b.sections {
		if s.Name == name {
			b.sections[i].Enabled = enabled
			return
		}
	}
}

// Build assembles the system prompt from all enabled sections.
func (b *SystemPromptBuilder) Build() string {
	// Sort by order
	sorted := make([]SystemPromptSection, len(b.sections))
	copy(sorted, b.sections)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Order < sorted[i].Order {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var parts []string
	for _, s := range sorted {
		if !s.Enabled || strings.TrimSpace(s.Content) == "" {
			continue
		}
		parts = append(parts, s.Content)
	}
	return strings.Join(parts, "\n\n")
}

// Reset clears all sections.
func (b *SystemPromptBuilder) Reset() {
	b.sections = nil
}

// SectionNames returns the names of all sections.
func (b *SystemPromptBuilder) SectionNames() []string {
	var names []string
	for _, s := range b.sections {
		names = append(names, s.Name)
	}
	return names
}

// DefaultSections returns the default system prompt sections.
func DefaultSections(sysPrompt string, skills []Skill, tools []string) []SystemPromptSection {
	var sections []SystemPromptSection

	// Identity
	if sysPrompt != "" {
		sections = append(sections, SystemPromptSection{
			Name:    "identity",
			Content: sysPrompt,
			Order:   0,
			Enabled: true,
		})
	} else {
		sections = append(sections, SystemPromptSection{
			Name:    "identity",
			Content: "You are rho, a helpful coding assistant.",
			Order:   0,
			Enabled: true,
		})
	}

	// Skills
	if len(skills) > 0 {
		sections = append(sections, SystemPromptSection{
			Name:    "skills",
			Content: FormatSkillsForPrompt(skills),
			Order:   10,
			Enabled: true,
		})
	}

	// Tools
	if len(tools) > 0 {
		toolList := "\n## Available Tools\n"
		for _, t := range tools {
			toolList += "- " + t + "\n"
		}
		sections = append(sections, SystemPromptSection{
			Name:    "tools",
			Content: toolList,
			Order:   20,
			Enabled: true,
		})
	}

	// Guidelines
	sections = append(sections, SystemPromptSection{
		Name: "guidelines",
		Content: `## Guidelines
- Read files before editing them to understand context.
- Use Bash for shell commands. Set timeout for long-running commands.
- Use Edit for exact text replacements in existing files.
- Use Write for new files or complete overwrites.
- Use Grep to search file contents.
- Use Find to locate files by name.
- Think step by step before making changes.`,
		Order:   30,
		Enabled: true,
	})

	return sections
}

// BuildSystemPrompt builds a complete system prompt.
func BuildSystemPrompt(sysPrompt string, skills []Skill, toolNames []string, additionalSections map[string]string) string {
	builder := NewSystemPromptBuilder()

	sections := DefaultSections(sysPrompt, skills, toolNames)
	for _, s := range sections {
		builder.sections = append(builder.sections, s)
	}

	// Add custom sections
	for name, content := range additionalSections {
		builder.AddSection(name, content, 50)
	}

	return builder.Build()
}

// BuildSystemPromptWithContext builds a system prompt with environment context.
func BuildSystemPromptWithContext(sysPrompt, cwd, shell string, skills []Skill, toolNames []string) string {
	envSection := fmt.Sprintf("## Environment\n- Working directory: %s\n- Shell: %s", cwd, shell)
	return BuildSystemPrompt(sysPrompt, skills, toolNames, map[string]string{
		"environment": envSection,
	})
}

// GenerateBranchSummary generates a summary of messages for a branch entry.
func GenerateBranchSummary(messages []agent.AgentMessage, maxLen int) string {
	if len(messages) == 0 {
		return "No messages to summarize."
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Branch with %d messages:\n\n", len(messages)))

	for i, msg := range messages {
		if b.Len() >= maxLen {
			b.WriteString("\n... (truncated)")
			break
		}

		rolePrefix := ""
		switch msg.Role {
		case "user":
			rolePrefix = "User"
		case "assistant":
			rolePrefix = "Assistant"
		case "toolResult":
			rolePrefix = "Tool"
		}

		content := msg.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}

		line := fmt.Sprintf("%d. [%s] %s\n", i+1, rolePrefix, content)
		b.WriteString(line)
	}

	result := b.String()
	if len(result) > maxLen {
		result = result[:maxLen] + "\n... (truncated)"
	}

	return result
}
