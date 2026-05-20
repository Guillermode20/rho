package codecore

import (
	"fmt"
	"strings"

	"github.com/earendil-works/rho/pkg/ai"
)

// ScopedModel represents a model with its scope (which context it was resolved from).
type ScopedModel struct {
	Model      ai.Model `json:"model"`
	Source     string   `json:"source"` // "cli", "config", "session", "default"
	Scope      string   `json:"scope"`  // "project", "user", "global"
	ProviderID string   `json:"providerId,omitempty"`
	Label      string   `json:"label,omitempty"`
}

// ModelScope describes the level at which a model preference is set.
type ModelScope string

const (
	ModelScopeCLI     ModelScope = "cli"
	ModelScopeConfig  ModelScope = "config"
	ModelScopeSession ModelScope = "session"
	ModelScopeDefault ModelScope = "default"
)

// ResolveModelResult holds the result of model resolution.
type ResolveModelResult struct {
	Model  ai.Model   `json:"model"`
	Scope  ModelScope `json:"scope"`
	Source string     `json:"source"` // description of where it came from
}

// ResolveCLIModel resolves a model from CLI arguments.
// Supports "provider/model", just "model", or partial matches.
func ResolveCLIModel(registry *ModelRegistry, modelArg, providerArg string) (*ResolveModelResult, error) {
	if modelArg == "" && providerArg == "" {
		return nil, fmt.Errorf("no model specified")
	}

	// Try full "provider/model" first
	if modelArg != "" && providerArg != "" {
		key := providerArg + "/" + modelArg
		if m, ok := registry.GetModel(ai.Provider(providerArg), modelArg); ok {
			return &ResolveModelResult{
				Model:  m,
				Scope:  ModelScopeCLI,
				Source: fmt.Sprintf("--model %s --provider %s", key, providerArg),
			}, nil
		}
		_ = key
	}

	// Try "provider/model" string
	if strings.Contains(modelArg, "/") {
		parts := strings.SplitN(modelArg, "/", 2)
		provStr, nameStr := parts[0], parts[1]
		if m, ok := registry.GetModel(ai.Provider(provStr), nameStr); ok {
			return &ResolveModelResult{
				Model:  m,
				Scope:  ModelScopeCLI,
				Source: fmt.Sprintf("--model %s", modelArg),
			}, nil
		}
	}

	// Try model name alone
	if modelArg != "" {
		if m, ok := registry.GetModelByName(modelArg); ok {
			return &ResolveModelResult{
				Model:  m,
				Scope:  ModelScopeCLI,
				Source: fmt.Sprintf("--model %s", modelArg),
			}, nil
		}
	}

	// Try provider alone - get first model for that provider
	if providerArg != "" {
		models := registry.GetModelsByProvider(ai.Provider(providerArg))
		if len(models) > 0 {
			return &ResolveModelResult{
				Model:  models[0],
				Scope:  ModelScopeCLI,
				Source: fmt.Sprintf("--provider %s (first model)", providerArg),
			}, nil
		}
	}

	// Partial match
	if m, ok := registry.FindModel(modelArg); ok {
		return &ResolveModelResult{
			Model:  m,
			Scope:  ModelScopeCLI,
			Source: fmt.Sprintf("partial match: '%s'", modelArg),
		}, nil
	}

	return nil, fmt.Errorf("could not resolve model %q (provider: %q)", modelArg, providerArg)
}

// ResolveModelScope determines the most specific scope for a model.
// Priority: CLI > Session > Config > Default
func ResolveModelScope(cliModel, sessionModel, configModel string) ModelScope {
	if cliModel != "" {
		return ModelScopeCLI
	}
	if sessionModel != "" {
		return ModelScopeSession
	}
	if configModel != "" {
		return ModelScopeConfig
	}
	return ModelScopeDefault
}

// AutoDetectProviderFromModel attempts to determine the provider from a model name.
// Uses common prefixes and patterns.
func AutoDetectProviderFromModel(modelName string) ai.Provider {
	name := strings.ToLower(modelName)
	switch {
	case strings.Contains(name, "claude"):
		return ai.ProviderAnthropic
	case strings.Contains(name, "gpt"), strings.Contains(name, "o1"), strings.Contains(name, "o3"):
		return ai.ProviderOpenAI
	case strings.Contains(name, "gemini"):
		return ai.ProviderGoogle
	case strings.Contains(name, "deepseek"):
		return ai.ProviderDeepSeek
	case strings.Contains(name, "glm"), strings.Contains(name, "kimi"), strings.Contains(name, "qwen"):
		return ai.ProviderCrof
	case strings.Contains(name, "mistral"):
		return ai.ProviderMistral
	case strings.Contains(name, "command"), strings.Contains(name, "cohere"):
		return ai.ProviderFireworks
	case strings.Contains(name, "llama"), strings.Contains(name, "mixtral"):
		return ai.ProviderGroq
	case strings.Contains(name, "bedrock"):
		return ai.ProviderAmazonBedrock
	default:
		return ""
	}
}

// AutoDetectAPIFromProvider returns the default API type for a provider.
func AutoDetectAPIFromProvider(provider ai.Provider) ai.API {
	switch provider {
	case ai.ProviderAnthropic:
		return ai.APIAnthropicMessages
	case ai.ProviderOpenAI, ai.ProviderDeepSeek, ai.ProviderGroq,
		ai.ProviderFireworks, ai.ProviderTogether, ai.ProviderXAI,
		ai.ProviderCerebras, ai.ProviderCrof, ai.ProviderOpenRouter, ai.ProviderVercelAIGateway:
		return ai.APIOpenAICompletions
	case ai.ProviderGoogle:
		return ai.APIGoogleGenerativeAI
	case ai.ProviderGoogleVertex:
		return ai.APIGoogleVertex
	case ai.ProviderMistral:
		return ai.APIMistralConversations
	case ai.ProviderAmazonBedrock:
		return ai.APIBedrockConverseStream
	case ai.ProviderAzureOpenAIResponses:
		return ai.APIAzureOpenAIResponses
	case ai.ProviderOpenAICodex:
		return ai.APIOpenAICodexResponses
	default:
		return ai.APIOpenAICompletions
	}
}

// FormatModel formats a model as "provider/name" for display.
func FormatModel(m ai.Model) string {
	return string(m.Provider) + "/" + m.Name
}

// FormatModelShort formats a model as just "name" when provider is known.
func FormatModelShort(m ai.Model, knownProvider ai.Provider) string {
	if m.Provider == knownProvider {
		return m.Name
	}
	return FormatModel(m)
}
