// Package systemprompt builds structured system prompts from multiple sources.
package systemprompt

import (
	"fmt"
	"strings"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/skills"
)

// BuildOptions configures how the system prompt is assembled.
type BuildOptions struct {
	// User-provided system prompt (highest priority)
	UserPrompt string `json:"userPrompt,omitempty"`
	// Base system instruction template
	BaseTemplate string `json:"baseTemplate,omitempty"`
	// Skills to include
	Skills []skills.Skill `json:"skills,omitempty"`
	// Project context (AGENTS.md, CLAUDE.md, etc.)
	ProjectContext string `json:"projectContext,omitempty"`
	// Available tools
	Tools []agent.AgentTool `json:"tools,omitempty"`
	// Custom instructions from extensions
	ExtensionInstructions []string `json:"extensionInstructions,omitempty"`
	// Working directory
	CWD string `json:"cwd,omitempty"`
	// Model info
	ModelName    string `json:"modelName,omitempty"`
	ProviderName string `json:"providerName,omitempty"`
}

// DefaultBaseTemplate returns the default system prompt template.
func DefaultBaseTemplate() string {
	return `You are rho, a lightweight coding agent. You help users with programming tasks by executing tools and providing guidance.

## Core Capabilities
- You can read, write, and edit files in the project
- You can execute bash commands to run code, install dependencies, etc.
- You can search file contents and find files
- You can list directory contents

## Guidelines
- Always check the current state before making changes
- Prefer reading files before editing them
- Use bash commands to verify your work
- Be concise and direct in your responses
- If you're unsure about something, ask the user

## Tool Usage
- Call tools one at a time when each depends on the previous result
- You can call multiple independent tools in parallel
- Always provide clear reasoning before using a tool
- Use the appropriate tool for each task`
}

// Build assembles the complete system prompt.
func Build(opts BuildOptions) string {
	var sb strings.Builder

	// Base template
	base := opts.BaseTemplate
	if base == "" {
		base = DefaultBaseTemplate()
	}
	sb.WriteString(base)
	sb.WriteString("\n\n")

	// Project context
	if opts.ProjectContext != "" {
		sb.WriteString("## Project Context\n\n")
		sb.WriteString(opts.ProjectContext)
		sb.WriteString("\n\n")
	}

	// Working directory
	if opts.CWD != "" {
		sb.WriteString(fmt.Sprintf("## Working Directory\n\n%s\n\n", opts.CWD))
	}

	// Skills
	if len(opts.Skills) > 0 {
		skillText := skills.FormatSkillsForPrompt(opts.Skills)
		if skillText != "" {
			sb.WriteString(skillText)
			sb.WriteString("\n\n")
		}
	}

	// Tools
	if len(opts.Tools) > 0 {
		sb.WriteString("## Available Tools\n\n")
		for _, t := range opts.Tools {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", t.Name, t.Description))
		}
		sb.WriteString("\n")
	}

	// Extension instructions
	for _, inst := range opts.ExtensionInstructions {
		sb.WriteString(inst)
		sb.WriteString("\n")
	}

	// Model/Provider info
	if opts.ModelName != "" {
		sb.WriteString(fmt.Sprintf("## Current Configuration\n\n"))
		sb.WriteString(fmt.Sprintf("- Model: %s\n", opts.ModelName))
		if opts.ProviderName != "" {
			sb.WriteString(fmt.Sprintf("- Provider: %s\n", opts.ProviderName))
		}
		sb.WriteString("\n")
	}

	// User system prompt (highest priority, appended last)
	if opts.UserPrompt != "" {
		sb.WriteString("## User Instructions\n\n")
		sb.WriteString(opts.UserPrompt)
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String())
}

// TokenEstimate returns an approximate token count for the built prompt.
func TokenEstimate(prompt string) int {
	// Rough estimate: ~4 chars per token
	return len(prompt)/4 + 1
}

// FormatToolList formats tools for the system prompt.
func FormatToolList(tools []agent.AgentTool) string {
	if len(tools) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Available tools:\n")
	for _, t := range tools {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Description))
	}
	return sb.String()
}
