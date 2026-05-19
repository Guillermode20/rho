package providers

import (
	"github.com/earendil-works/rho/pkg/ai"
)

// BuildBaseOptions builds a StreamOptions from a SimpleStreamOptions.
// This copies the common fields and is the starting point for provider-specific
// option builders.
func BuildBaseOptions(model ai.Model, options *ai.SimpleStreamOptions, apiKey string) *ai.StreamOptions {
	if options == nil {
		return &ai.StreamOptions{
			APIKey: apiKey,
		}
	}

	base := &ai.StreamOptions{
		Temperature:    options.Temperature,
		MaxTokens:      options.MaxTokens,
		APIKey:         apiKey,
		Transport:      options.Transport,
		CacheRetention: options.CacheRetention,
		SessionID:      options.SessionID,
		Headers:        options.Headers,
		TimeoutMs:      options.TimeoutMs,
		MaxRetries:     options.MaxRetries,
		MaxRetryDelayMs: options.MaxRetryDelayMs,
		Metadata:       options.Metadata,
	}

	// Use the provided API key if no explicit override
	if base.APIKey == "" {
		base.APIKey = apiKey
	}

	// Copy signal if present
	if options.Signal != nil {
		base.Signal = options.Signal
	}

	return base
}

// ClampReasoning maps an extended thinking level to a provider-supported level.
// "xhigh" is clamped to "high" for providers that don't support it.
func ClampReasoning(effort ai.ThinkingLevel) ai.ThinkingLevel {
	if effort == "xhigh" {
		return "high"
	}
	return effort
}

// DefaultThinkingBudgets returns the default token budgets for each thinking level.
// These are used to calculate max_tokens for providers with thinking budgets.
func DefaultThinkingBudgets() map[ai.ThinkingLevel]int {
	return map[ai.ThinkingLevel]int{
		"minimal": 1024,
		"low":     2048,
		"medium":  8192,
		"high":    16384,
	}
}

// ThinkingBudgetResult holds the result of thinking budget calculations.
type ThinkingBudgetResult struct {
	MaxTokens      int
	ThinkingBudget int
}

// AdjustMaxTokensForThinking calculates the proper max_tokens and thinking budget
// when a model has thinking/reasoning enabled. It reserves tokens for the thinking
// budget within the overall max_tokens limit.
//
// Parameters:
//   - baseMaxTokens: user-specified max tokens (0 = use model cap)
//   - modelMaxTokens: the model's maximum output token limit
//   - reasoningLevel: the reasoning/thinking effort level
//   - customBudgets: optional custom token budgets per level
func AdjustMaxTokensForThinking(
	baseMaxTokens int,
	modelMaxTokens int,
	reasoningLevel ai.ThinkingLevel,
	customBudgets map[ai.ThinkingLevel]int,
) ThinkingBudgetResult {
	budgets := DefaultThinkingBudgets()
	for k, v := range customBudgets {
		budgets[k] = v
	}

	clampedLevel := ClampReasoning(reasoningLevel)
	thinkingBudget := budgets[clampedLevel]

	// Ensure at least 1024 tokens for output beyond thinking
	const minOutputTokens = 1024

	// If no user cap, use model cap
	maxTokens := modelMaxTokens
	if baseMaxTokens > 0 {
		maxTokens = baseMaxTokens + thinkingBudget
		if maxTokens > modelMaxTokens {
			maxTokens = modelMaxTokens
		}
	}

	// If the thinking budget exceeds available tokens, reduce it
	if maxTokens <= thinkingBudget {
		thinkingBudget = maxTokens - minOutputTokens
		if thinkingBudget < 0 {
			thinkingBudget = 0
		}
	}

	return ThinkingBudgetResult{
		MaxTokens:      maxTokens,
		ThinkingBudget: thinkingBudget,
	}
}
