package codecore

import (
	"github.com/earendil-works/rho/pkg/ai"
)

// DefaultModelName is the default model identifier.
const DefaultModelName = "claude-sonnet-4-20250514"

// DefaultProvider is the default AI provider.
const DefaultProvider = "anthropic"

// DefaultMaxTokens is the default maximum output tokens.
const DefaultMaxTokens = 8192

// DefaultTemperature is the default generation temperature.
const DefaultTemperature = 0.0

// DefaultSystemPrompt is the default system prompt for the coding agent.
const DefaultSystemPrompt = `You are rho, a lightweight coding agent. You help users with software development tasks.

You have access to a set of tools that you can use to accomplish tasks. Use them when appropriate.

When writing code:
- Write clean, well-documented code
- Follow the existing code style
- Create appropriate error handling
- Consider edge cases

When running commands:
- Prefer safe commands that don't modify the system
- Ask for confirmation before destructive operations
- Use relative paths when possible`

// DefaultConfig returns the default runtime configuration values.
func DefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"model":           DefaultModelName,
		"provider":        DefaultProvider,
		"maxTokens":       DefaultMaxTokens,
		"temperature":     DefaultTemperature,
		"systemPrompt":    DefaultSystemPrompt,
		"toolExecutionMode": "sequential",
		"retry.enabled":   true,
		"retry.maxRetries": 3,
	}
}

// DefaultModel returns the default AI model.
func DefaultModel() ai.Model {
	return ai.Model{
		API:      ai.APIAnthropicMessages,
		Provider: ai.ProviderAnthropic,
		Name:     DefaultModelName,
	}
}

// DefaultToolSettings returns default tool configuration.
func DefaultToolSettings() map[string]interface{} {
	return map[string]interface{}{
		"read.maxBytes":   50000,
		"read.maxLines":   2000,
		"bash.maxOutput":  100000,
		"bash.timeout":    30,
		"grep.maxMatches": 100,
		"find.maxResults": 100,
		"ls.maxEntries":   500,
	}
}

// DefaultThinkingLevel returns the default thinking level.
func DefaultThinkingLevel() string {
	return "off"
}

// DefaultCompactionSettings returns default compaction configuration.
func DefaultCompactionSettings() map[string]interface{} {
	return map[string]interface{}{
		"enabled":         true,
		"thresholdTokens": 100000,
		"minTokens":       50000,
		"targetTokens":    75000,
	}
}

// DefaultSessionSettings returns default session configuration.
func DefaultSessionSettings() map[string]interface{} {
	return map[string]interface{}{
		"autoSave":     true,
		"saveInterval": 60000, // 1 minute
		"maxSessions":  100,
		"pruneAfter":   30, // days
	}
}
