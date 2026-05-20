package ai

import "os"

// CostPerMillion is the cost per 1M tokens for a model.
type CostPerMillion struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

// ModelDefinition describes a model's capabilities.
type ModelDefinition struct {
	API            API            `json:"api"`
	Provider       Provider       `json:"provider"`
	Name           string         `json:"name"`
	BaseURL        string         `json:"baseUrl,omitempty"`
	ContextWindow  int            `json:"contextWindow"`
	MaxTokens      int            `json:"maxTokens"`
	Reasoning      bool           `json:"reasoning"`
	Input          []string       `json:"input"` // "text", "image", "code" etc.
	Cost           CostPerMillion `json:"cost"`
	ThinkingLevels []string       `json:"thinkingLevels,omitempty"`
	Description    string         `json:"description,omitempty"`
}

// DefaultModels returns the built-in model definitions.
func DefaultModels() []ModelDefinition {
	return []ModelDefinition{
		// ── Anthropic ──
		{API: APIAnthropicMessages, Provider: ProviderAnthropic, Name: "claude-sonnet-4-20250514", ContextWindow: 200000, MaxTokens: 8192, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 3, Output: 15, CacheWrite: 3.75, CacheRead: 0.30}},
		{API: APIAnthropicMessages, Provider: ProviderAnthropic, Name: "claude-3-5-sonnet-20241022", ContextWindow: 200000, MaxTokens: 8192, Reasoning: false, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 3, Output: 15, CacheWrite: 3.75, CacheRead: 0.30}},
		{API: APIAnthropicMessages, Provider: ProviderAnthropic, Name: "claude-3-opus-20240229", ContextWindow: 200000, MaxTokens: 4096, Reasoning: false, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 15, Output: 75, CacheWrite: 18.75, CacheRead: 1.50}},
		{API: APIAnthropicMessages, Provider: ProviderAnthropic, Name: "claude-3-haiku-20240307", ContextWindow: 200000, MaxTokens: 4096, Reasoning: false, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 0.25, Output: 1.25, CacheWrite: 0.30, CacheRead: 0.03}},

		// ── OpenAI ──
		{API: APIOpenAICompletions, Provider: ProviderOpenAI, Name: "gpt-4o", ContextWindow: 128000, MaxTokens: 16384, Reasoning: false, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 2.50, Output: 10, CacheRead: 1.25}},
		{API: APIOpenAICompletions, Provider: ProviderOpenAI, Name: "gpt-4o-mini", ContextWindow: 128000, MaxTokens: 16384, Reasoning: false, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 0.15, Output: 0.60, CacheRead: 0.075}},
		{API: APIOpenAICompletions, Provider: ProviderOpenAI, Name: "o3-mini", ContextWindow: 200000, MaxTokens: 100000, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 1.10, Output: 4.40, CacheRead: 0.55}},

		// ── Google ──
		{API: APIGoogleGenerativeAI, Provider: ProviderGoogle, Name: "gemini-2.5-pro-exp-03-25", ContextWindow: 1000000, MaxTokens: 8192, Reasoning: true, Input: []string{"text", "image", "audio", "video"}, Cost: CostPerMillion{Input: 1.25, Output: 10}},
		{API: APIGoogleGenerativeAI, Provider: ProviderGoogle, Name: "gemini-2.0-flash", ContextWindow: 1000000, MaxTokens: 8192, Reasoning: false, Input: []string{"text", "image", "audio", "video"}, Cost: CostPerMillion{Input: 0.10, Output: 0.40}},

		// ── Google Vertex AI ──
		{API: APIGoogleVertex, Provider: ProviderGoogleVertex, Name: "gemini-2.5-pro-exp-03-25", ContextWindow: 1000000, MaxTokens: 8192, Reasoning: true, Input: []string{"text", "image", "audio", "video"}, Cost: CostPerMillion{Input: 1.25, Output: 10}},
		{API: APIGoogleVertex, Provider: ProviderGoogleVertex, Name: "gemini-2.0-flash", ContextWindow: 1000000, MaxTokens: 8192, Reasoning: false, Input: []string{"text", "image", "audio", "video"}, Cost: CostPerMillion{Input: 0.10, Output: 0.40}},

		// ── DeepSeek (OpenAI-compatible) ──
		{API: APIOpenAICompletions, Provider: ProviderDeepSeek, Name: "deepseek-chat", BaseURL: "https://api.deepseek.com", ContextWindow: 64000, MaxTokens: 8192, Reasoning: false, Input: []string{"text"}, Cost: CostPerMillion{Input: 0.27, Output: 1.10}},

		// ── CrofAI (OpenAI-compatible) ──
		{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "glm-5.1", BaseURL: "https://crof.ai", ContextWindow: 202000, MaxTokens: 32768, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 0.45, Output: 2.10}},
		{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "glm-5.1-precision", BaseURL: "https://crof.ai", ContextWindow: 202000, MaxTokens: 32768, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 0.80, Output: 2.90}},
		{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "kimi-k2.6", BaseURL: "https://crof.ai", ContextWindow: 262000, MaxTokens: 32768, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 0.50, Output: 1.99}},
		{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "kimi-k2.6-precision", BaseURL: "https://crof.ai", ContextWindow: 262000, MaxTokens: 32768, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 0.50, Output: 1.99}},
		{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "deepseek-v4-pro", BaseURL: "https://crof.ai", ContextWindow: 163000, MaxTokens: 32768, Reasoning: true, Input: []string{"text"}, Cost: CostPerMillion{Input: 1.00, Output: 2.15}},
		{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "deepseek-v3.2", BaseURL: "https://crof.ai", ContextWindow: 163000, MaxTokens: 32768, Reasoning: true, Input: []string{"text"}, Cost: CostPerMillion{Input: 0.28, Output: 0.38}},
		{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "qwen3.5-397b-a17b", BaseURL: "https://crof.ai", ContextWindow: 262000, MaxTokens: 32768, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{}},
		{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "minimax-m2.5", BaseURL: "https://crof.ai", ContextWindow: 262000, MaxTokens: 32768, Reasoning: true, Input: []string{"text"}, Cost: CostPerMillion{}},
		{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "glm-4.7-flash", BaseURL: "https://crof.ai", ContextWindow: 202000, MaxTokens: 32768, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{}},

		// ── Mistral ──
		{API: APIMistralConversations, Provider: ProviderMistral, Name: "mistral-large-2411", ContextWindow: 128000, MaxTokens: 8192, Reasoning: false, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 2, Output: 6}},

		// ── Amazon Bedrock ──
		{API: APIBedrockConverseStream, Provider: ProviderAmazonBedrock, Name: "claude-sonnet-4-20250514", ContextWindow: 200000, MaxTokens: 8192, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 3, Output: 15}},
		{API: APIBedrockConverseStream, Provider: ProviderAmazonBedrock, Name: "claude-3-haiku-20240307", ContextWindow: 200000, MaxTokens: 4096, Reasoning: false, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 0.25, Output: 1.25}},

		// ── Azure OpenAI ──
		{API: APIAzureOpenAIResponses, Provider: ProviderAzureOpenAIResponses, Name: "gpt-4o", ContextWindow: 128000, MaxTokens: 16384, Reasoning: false, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 2.50, Output: 10}},

		// ── xAI (OpenAI-compatible) ──
		{API: APIOpenAICompletions, Provider: ProviderXAI, Name: "grok-2", BaseURL: "https://api.x.ai/v1", ContextWindow: 131072, MaxTokens: 8192, Reasoning: false, Input: []string{"text"}, Cost: CostPerMillion{Input: 2, Output: 10}},

		// ── Groq (OpenAI-compatible) ──
		{API: APIOpenAICompletions, Provider: ProviderGroq, Name: "llama-3.3-70b-versatile", BaseURL: "https://api.groq.com/openai/v1", ContextWindow: 131072, MaxTokens: 8192, Reasoning: false, Input: []string{"text"}, Cost: CostPerMillion{Input: 0.59, Output: 0.79}},

		// ── Cerebras (OpenAI-compatible) ──
		{API: APIOpenAICompletions, Provider: ProviderCerebras, Name: "llama-3.1-8b", BaseURL: "https://api.cerebras.ai/v1", ContextWindow: 8192, MaxTokens: 4096, Reasoning: false, Input: []string{"text"}, Cost: CostPerMillion{Input: 0.10, Output: 0.10}},

		// ── OpenRouter (OpenAI-compatible) ──
		{API: APIOpenAICompletions, Provider: ProviderOpenRouter, Name: "auto", BaseURL: "https://openrouter.ai/api/v1", ContextWindow: 128000, MaxTokens: 8192, Reasoning: false, Input: []string{"text"}, Cost: CostPerMillion{Input: 0, Output: 0}},

		// ── Fireworks (OpenAI-compatible) ──
		{API: APIOpenAICompletions, Provider: ProviderFireworks, Name: "llama-v3p3-70b-instruct", BaseURL: "https://api.fireworks.ai/inference/v1", ContextWindow: 131072, MaxTokens: 8192, Reasoning: false, Input: []string{"text"}, Cost: CostPerMillion{Input: 0.90, Output: 0.90}},

		// ── Together (OpenAI-compatible) ──
		{API: APIOpenAICompletions, Provider: ProviderTogether, Name: "llama-3.3-70b-instruct-turbo", BaseURL: "https://api.together.xyz/v1", ContextWindow: 131072, MaxTokens: 8192, Reasoning: false, Input: []string{"text"}, Cost: CostPerMillion{Input: 0.88, Output: 0.88}},

		// ── GitHub Copilot (OpenAI-compatible) ──
		{API: APIOpenAICompletions, Provider: ProviderGitHubCopilot, Name: "gpt-4o", ContextWindow: 128000, MaxTokens: 16384, Reasoning: false, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 0, Output: 0}},

		// ── Vercel AI Gateway (OpenAI-compatible) ──
		{API: APIOpenAICompletions, Provider: ProviderVercelAIGateway, Name: "auto", BaseURL: "https://gateway.ai.vercel.ai/v1", ContextWindow: 128000, MaxTokens: 8192, Reasoning: false, Input: []string{"text"}, Cost: CostPerMillion{Input: 0, Output: 0}},

		// ── OpenAI Codex (for Copilot/ChatGPT) ──
		{API: APIOpenAICodexResponses, Provider: ProviderOpenAICodex, Name: "gpt-4o", ContextWindow: 128000, MaxTokens: 16384, Reasoning: false, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 0, Output: 0}},

		// ── Cloudflare AI Gateway ──
		{API: APIOpenAICompletions, Provider: ProviderCloudflareAIGateway, Name: "auto", BaseURL: "https://gateway.ai.cloudflare.com/v1", ContextWindow: 128000, MaxTokens: 8192, Reasoning: false, Input: []string{"text"}, Cost: CostPerMillion{Input: 0, Output: 0}},

		// ── Cloudflare Workers AI ──
		{API: APIOpenAICompletions, Provider: ProviderCloudflareWorkersAI, Name: "auto", BaseURL: "https://api.cloudflare.com/client/v4/accounts", ContextWindow: 128000, MaxTokens: 8192, Reasoning: false, Input: []string{"text"}, Cost: CostPerMillion{Input: 0, Output: 0}},
	}
}

// providerEnvKeys maps provider names to their expected environment variable names.
func providerEnvKeys(provider Provider) []string {
	switch provider {
	case ProviderAnthropic:
		return []string{"ANTHROPIC_API_KEY", "CLAUDE_API_KEY"}
	case ProviderOpenAI:
		return []string{"OPENAI_API_KEY"}
	case ProviderGoogle:
		return []string{"GOOGLE_API_KEY", "GEMINI_API_KEY"}
	case ProviderDeepSeek:
		return []string{"DEEPSEEK_API_KEY"}
	case ProviderCrof:
		return []string{"CROF_API_KEY", "CROFAI_API_KEY"}
	case ProviderMistral:
		return []string{"MISTRAL_API_KEY"}
	case ProviderGroq:
		return []string{"GROQ_API_KEY"}
	case ProviderCerebras:
		return []string{"CEREBRAS_API_KEY"}
	case ProviderXAI:
		return []string{"XAI_API_KEY"}
	case ProviderOpenRouter:
		return []string{"OPENROUTER_API_KEY"}
	case ProviderFireworks:
		return []string{"FIREWORKS_API_KEY"}
	case ProviderTogether:
		return []string{"TOGETHER_API_KEY"}
	case ProviderGitHubCopilot:
		return []string{"COPILOT_GITHUB_TOKEN"}
	case ProviderOpenAICodex:
		return []string{"OPENAI_API_KEY"}
	case ProviderAzureOpenAIResponses:
		return []string{"AZURE_OPENAI_API_KEY"}
	case ProviderVercelAIGateway:
		return []string{"AI_GATEWAY_API_KEY"}
	case ProviderCloudflareAIGateway, ProviderCloudflareWorkersAI:
		return []string{"CLOUDFLARE_API_KEY"}
	default:
		return nil
	}
}

// ProviderHasEnvKey checks if any of the known environment variables for a provider are set.
func ProviderHasEnvKey(provider Provider) bool {
	for _, key := range providerEnvKeys(provider) {
		if v := os.Getenv(key); v != "" {
			return true
		}
	}
	return false
}

// AvailableModels filters DefaultModels to only those whose providers have configured authentication.
// authCheck is called for each unique provider; return true if auth is configured.
func AvailableModels(authCheck func(provider Provider) bool) []ModelDefinition {
	var result []ModelDefinition
	for _, m := range DefaultModels() {
		if authCheck(m.Provider) {
			result = append(result, m)
		}
	}
	return result
}

// CalculateCost calculates the cost for a given usage and model.
func CalculateCost(model ModelDefinition, usage Usage) Cost {
	inputCost := (model.Cost.Input / 1000000) * float64(usage.Input)
	outputCost := (model.Cost.Output / 1000000) * float64(usage.Output)
	cacheReadCost := (model.Cost.CacheRead / 1000000) * float64(usage.CacheRead)
	cacheWriteCost := (model.Cost.CacheWrite / 1000000) * float64(usage.CacheWrite)
	return Cost{
		Input:      inputCost,
		Output:     outputCost,
		CacheRead:  cacheReadCost,
		CacheWrite: cacheWriteCost,
		Total:      inputCost + outputCost + cacheReadCost + cacheWriteCost,
	}
}

// ModelRegistry built-in initialization.
func init() {
	// Register default models from provider registry
	for _, def := range DefaultModels() {
		m := Model{
			API:      def.API,
			Provider: def.Provider,
			Name:     def.Name,
		}
		// Auto-register in the default registry
		DefaultModelRegistry.RegisterModel(m)
	}
}

// DefaultModelRegistry is the package-level model registry.
var DefaultModelRegistry = NewModelRegistry()
