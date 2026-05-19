package ai

// CostPerMillion is the cost per 1M tokens for a model.
type CostPerMillion struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

// ModelDefinition describes a model's capabilities.
type ModelDefinition struct {
	API           API              `json:"api"`
	Provider      Provider         `json:"provider"`
	Name          string           `json:"name"`
	BaseURL       string           `json:"baseUrl,omitempty"`
	ContextWindow int              `json:"contextWindow"`
	MaxTokens     int              `json:"maxTokens"`
	Reasoning     bool             `json:"reasoning"`
	Input         []string         `json:"input"` // "text", "image", "code" etc.
	Cost          CostPerMillion   `json:"cost"`
	ThinkingLevels []string        `json:"thinkingLevels,omitempty"`
	Description   string           `json:"description,omitempty"`
}

// DefaultModels returns the built-in model definitions.
func DefaultModels() []ModelDefinition {
	return []ModelDefinition{
		// Anthropic models
		{API: APIAnthropicMessages, Provider: ProviderAnthropic, Name: "claude-sonnet-4-20250514", ContextWindow: 200000, MaxTokens: 8192, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 3, Output: 15, CacheWrite: 3.75, CacheRead: 0.30}},
		{API: APIAnthropicMessages, Provider: ProviderAnthropic, Name: "claude-3-5-sonnet-20241022", ContextWindow: 200000, MaxTokens: 8192, Reasoning: false, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 3, Output: 15, CacheWrite: 3.75, CacheRead: 0.30}},
		{API: APIAnthropicMessages, Provider: ProviderAnthropic, Name: "claude-3-opus-20240229", ContextWindow: 200000, MaxTokens: 4096, Reasoning: false, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 15, Output: 75, CacheWrite: 18.75, CacheRead: 1.50}},
		{API: APIAnthropicMessages, Provider: ProviderAnthropic, Name: "claude-3-haiku-20240307", ContextWindow: 200000, MaxTokens: 4096, Reasoning: false, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 0.25, Output: 1.25, CacheWrite: 0.30, CacheRead: 0.03}},

		// OpenAI models
		{API: APIOpenAICompletions, Provider: ProviderOpenAI, Name: "gpt-4o", ContextWindow: 128000, MaxTokens: 16384, Reasoning: false, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 2.50, Output: 10, CacheRead: 1.25}},
		{API: APIOpenAICompletions, Provider: ProviderOpenAI, Name: "gpt-4o-mini", ContextWindow: 128000, MaxTokens: 16384, Reasoning: false, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 0.15, Output: 0.60, CacheRead: 0.075}},
		{API: APIOpenAICompletions, Provider: ProviderOpenAI, Name: "o3-mini", ContextWindow: 200000, MaxTokens: 100000, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 1.10, Output: 4.40, CacheRead: 0.55}},

		// Google models
		{API: APIGoogleGenerativeAI, Provider: ProviderGoogle, Name: "gemini-2.5-pro-exp-03-25", ContextWindow: 1000000, MaxTokens: 8192, Reasoning: true, Input: []string{"text", "image", "audio", "video"}, Cost: CostPerMillion{Input: 1.25, Output: 10}},
		{API: APIGoogleGenerativeAI, Provider: ProviderGoogle, Name: "gemini-2.0-flash", ContextWindow: 1000000, MaxTokens: 8192, Reasoning: false, Input: []string{"text", "image", "audio", "video"}, Cost: CostPerMillion{Input: 0.10, Output: 0.40}},

		// DeepSeek (OpenAI-compatible)
		{API: APIOpenAICompletions, Provider: ProviderDeepSeek, Name: "deepseek-chat", BaseURL: "https://api.deepseek.com", ContextWindow: 64000, MaxTokens: 8192, Reasoning: false, Input: []string{"text"}, Cost: CostPerMillion{Input: 0.27, Output: 1.10}},

		// Mistral
		{API: APIMistralConversations, Provider: ProviderMistral, Name: "mistral-large-2411", ContextWindow: 128000, MaxTokens: 8192, Reasoning: false, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 2, Output: 6}},
	}
}

// CalculateCost calculates the cost for a given usage and model.
func CalculateCost(model ModelDefinition, usage Usage) Cost {
	inputCost := (model.Cost.Input / 1000000) * float64(usage.Input)
	outputCost := (model.Cost.Output / 1000000) * float64(usage.Output)
	cacheReadCost := (model.Cost.CacheRead / 1000000) * float64(usage.CacheRead)
	cacheWriteCost := (model.Cost.CacheWrite / 1000000) * float64(usage.CacheWrite)
	return Cost{
		Input:     inputCost,
		Output:    outputCost,
		CacheRead: cacheReadCost,
		CacheWrite: cacheWriteCost,
		Total:     inputCost + outputCost + cacheReadCost + cacheWriteCost,
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
