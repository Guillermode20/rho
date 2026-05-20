package ai

import (
	"os"
	"sync"
)

// CostPerMillion is the cost per 1M tokens for a model.
type CostPerMillion struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

// ModelDefinition describes a model's capabilities.
type ModelDefinition struct {
	API            API                `json:"api"`
	Provider       Provider           `json:"provider"`
	Name           string             `json:"name"`
	DisplayName    string             `json:"displayName,omitempty"`
	BaseURL        string             `json:"baseUrl,omitempty"`
	ContextWindow  int                `json:"contextWindow"`
	MaxTokens      int                `json:"maxTokens"`
	Reasoning      bool               `json:"reasoning"`
	Input          []string           `json:"input"` // "text", "image", "code" etc.
	Cost           CostPerMillion     `json:"cost"`
	ThinkingLevels []string           `json:"thinkingLevels,omitempty"`
	ThinkingMap    map[string]*string `json:"thinkingMap,omitempty"`
	Headers        map[string]string  `json:"headers,omitempty"`
	Description    string             `json:"description,omitempty"`
}

// ImageModelDefinition describes an image generation model.
type ImageModelDefinition struct {
	API         API            `json:"api"`
	Provider    Provider       `json:"provider"`
	Name        string         `json:"name"`
	DisplayName string         `json:"displayName,omitempty"`
	BaseURL     string         `json:"baseUrl,omitempty"`
	Input       []string       `json:"input"`
	Output      []string       `json:"output"`
	Cost        CostPerMillion `json:"cost"`
}

func pointerToString(s string) *string {
	return &s
}

// CustomCrofModels are CrofAI (via openai-compatible API) model definitions maintained separately
// from the generated models to avoid duplication across initModels and ResetActiveProviderModels.
var CustomCrofModels = []ModelDefinition{
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "glm-5.1", BaseURL: "https://crof.ai", ContextWindow: 202000, MaxTokens: 32768, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 0.45, Output: 2.10}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "glm-5.1-precision", BaseURL: "https://crof.ai", ContextWindow: 202000, MaxTokens: 32768, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 0.80, Output: 2.90}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "kimi-k2.6", BaseURL: "https://crof.ai", ContextWindow: 262000, MaxTokens: 32768, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 0.50, Output: 1.99}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "kimi-k2.6-precision", BaseURL: "https://crof.ai", ContextWindow: 262000, MaxTokens: 32768, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{Input: 0.50, Output: 1.99}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "deepseek-v4-pro", BaseURL: "https://crof.ai", ContextWindow: 163000, MaxTokens: 32768, Reasoning: true, Input: []string{"text"}, Cost: CostPerMillion{Input: 1.00, Output: 2.15}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "deepseek-v4-pro-precision", BaseURL: "https://crof.ai", ContextWindow: 163000, MaxTokens: 32768, Reasoning: true, Input: []string{"text"}, Cost: CostPerMillion{Input: 1.20, Output: 2.50}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "deepseek-v4-flash", BaseURL: "https://crof.ai", ContextWindow: 163000, MaxTokens: 32768, Reasoning: false, Input: []string{"text"}, Cost: CostPerMillion{Input: 0.14, Output: 0.28}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "deepseek-v3.2", BaseURL: "https://crof.ai", ContextWindow: 163000, MaxTokens: 32768, Reasoning: true, Input: []string{"text"}, Cost: CostPerMillion{Input: 0.28, Output: 0.38}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "mimo-v2.5-pro", BaseURL: "https://crof.ai", ContextWindow: 262000, MaxTokens: 32768, Reasoning: false, Input: []string{"text", "image"}, Cost: CostPerMillion{}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "mimo-v2.5-pro-precision", BaseURL: "https://crof.ai", ContextWindow: 262000, MaxTokens: 32768, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "qwen3.5-397b-a17b", BaseURL: "https://crof.ai", ContextWindow: 262000, MaxTokens: 32768, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "minimax-m2.5", BaseURL: "https://crof.ai", ContextWindow: 262000, MaxTokens: 32768, Reasoning: true, Input: []string{"text"}, Cost: CostPerMillion{}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "glm-5", BaseURL: "https://crof.ai", ContextWindow: 202000, MaxTokens: 32768, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "glm-4.7", BaseURL: "https://crof.ai", ContextWindow: 202000, MaxTokens: 32768, Reasoning: false, Input: []string{"text", "image"}, Cost: CostPerMillion{}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "glm-4.7-flash", BaseURL: "https://crof.ai", ContextWindow: 202000, MaxTokens: 32768, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "gemma-4-31b-it", BaseURL: "https://crof.ai", ContextWindow: 128000, MaxTokens: 32768, Reasoning: false, Input: []string{"text"}, Cost: CostPerMillion{}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "qwen3.6-27b", BaseURL: "https://crof.ai", ContextWindow: 128000, MaxTokens: 32768, Reasoning: false, Input: []string{"text"}, Cost: CostPerMillion{}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "qwen3.5-9b", BaseURL: "https://crof.ai", ContextWindow: 128000, MaxTokens: 32768, Reasoning: false, Input: []string{"text"}, Cost: CostPerMillion{}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "kimi-k2.5", BaseURL: "https://crof.ai", ContextWindow: 262000, MaxTokens: 32768, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{}},
	{API: APIOpenAICompletions, Provider: ProviderCrof, Name: "kimi-k2.5-lightning", BaseURL: "https://crof.ai", ContextWindow: 262000, MaxTokens: 32768, Reasoning: true, Input: []string{"text", "image"}, Cost: CostPerMillion{}},
}

var (
	activeModels   []ModelDefinition
	activeModelsMu sync.RWMutex
	onceInitModels sync.Once
)

func initModels() {
	onceInitModels.Do(func() {
		models := GeneratedModels()
		activeModels = append(models, CustomCrofModels...)
	})
}

// DefaultModels returns the built-in and dynamically fetched model definitions.
func DefaultModels() []ModelDefinition {
	initModels()
	activeModelsMu.RLock()
	defer activeModelsMu.RUnlock()
	res := make([]ModelDefinition, len(activeModels))
	copy(res, activeModels)
	return res
}

// UpdateActiveProviderModels thread-safely updates models for a provider in the active models list.
func UpdateActiveProviderModels(provider Provider, defs []ModelDefinition) {
	initModels()
	activeModelsMu.Lock()
	defer activeModelsMu.Unlock()

	var filtered []ModelDefinition
	for _, m := range activeModels {
		if m.Provider != provider {
			filtered = append(filtered, m)
		}
	}
	activeModels = append(filtered, defs...)
}

// ProviderEnvKeys maps provider names to their expected environment variable names.
func ProviderEnvKeys(provider Provider) []string {
	switch provider {
	case ProviderAnthropic:
		return []string{"ANTHROPIC_OAUTH_TOKEN", "ANTHROPIC_API_KEY", "CLAUDE_API_KEY"}
	case ProviderOpenAI:
		return []string{"OPENAI_API_KEY"}
	case ProviderGoogle:
		return []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"}
	case ProviderGoogleVertex:
		return []string{"GOOGLE_CLOUD_API_KEY", "GOOGLE_APPLICATION_CREDENTIALS"}
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
	case ProviderHuggingFace:
		return []string{"HF_TOKEN"}
	case ProviderKimiCoding:
		return []string{"KIMI_API_KEY"}
	case ProviderMinimax:
		return []string{"MINIMAX_API_KEY"}
	case ProviderMinimaxCN:
		return []string{"MINIMAX_CN_API_KEY"}
	case ProviderMoonshotAI, ProviderMoonshotAICN:
		return []string{"MOONSHOT_API_KEY"}
	case ProviderZAI:
		return []string{"ZAI_API_KEY"}
	case ProviderOpenCode, ProviderOpenCodeGo:
		return []string{"OPENCODE_API_KEY"}
	case ProviderXiaomi:
		return []string{"XIAOMI_API_KEY"}
	case ProviderXiaomiTokenPlanCN:
		return []string{"XIAOMI_TOKEN_PLAN_CN_API_KEY"}
	case ProviderXiaomiTokenPlanAMS:
		return []string{"XIAOMI_TOKEN_PLAN_AMS_API_KEY"}
	case ProviderXiaomiTokenPlanSGP:
		return []string{"XIAOMI_TOKEN_PLAN_SGP_API_KEY"}
	default:
		return nil
	}
}

// ProviderPriority returns a priority number for a provider (lower = preferred).
// Used when multiple providers define the same model name.
func ProviderPriority(p Provider) int {
	switch p {
	case ProviderAnthropic:
		return 1
	case ProviderOpenAI:
		return 2
	case ProviderGoogle:
		return 3
	case ProviderDeepSeek:
		return 4
	case ProviderMistral:
		return 5
	case ProviderGroq:
		return 6
	case ProviderCerebras:
		return 7
	case ProviderXAI:
		return 8
	case ProviderHuggingFace:
		return 9
	case ProviderOpenRouter:
		return 20
	case ProviderAzureOpenAIResponses:
		return 30
	case ProviderAmazonBedrock:
		return 40
	default:
		return 100
	}
}

// ProviderHasEnvKey checks if any of the known environment variables for a provider are set.
func ProviderHasEnvKey(provider Provider) bool {
	for _, key := range ProviderEnvKeys(provider) {
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
			BaseURL:  def.BaseURL,
			Headers:  def.Headers,
		}
		// Auto-register in the default registry
		DefaultModelRegistry.RegisterModel(m)
	}
}

// DefaultModelRegistry is the package-level model registry.
var DefaultModelRegistry = NewModelRegistry()

// ResetActiveProviderModels resets the provider's models back to the built-in defaults.
func ResetActiveProviderModels(provider Provider) {
	initModels()
	activeModelsMu.Lock()
	defer activeModelsMu.Unlock()

	var filtered []ModelDefinition
	for _, m := range activeModels {
		if m.Provider != provider {
			filtered = append(filtered, m)
		}
	}

	var defaults []ModelDefinition
	for _, m := range GeneratedModels() {
		if m.Provider == provider {
			defaults = append(defaults, m)
		}
	}
	if provider == ProviderCrof {
		defaults = append(defaults, CustomCrofModels...)
	}
	activeModels = append(filtered, defaults...)
}
