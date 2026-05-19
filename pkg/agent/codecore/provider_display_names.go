package codecore

import (
	"github.com/earendil-works/rho/pkg/ai"
)

// ProviderDisplayInfo holds display information for a provider.
type ProviderDisplayInfo struct {
	Name        string `json:"name"`
	ShortName   string `json:"shortName"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
	URL         string `json:"url"`
	DocsURL     string `json:"docsUrl"`
}

// providerDisplayNames maps providers to their display information.
var providerDisplayNames = map[ai.Provider]ProviderDisplayInfo{
	ai.ProviderAnthropic: {
		Name:        "Anthropic",
		ShortName:   "Anthropic",
		Icon:        "●",
		Description: "Anthropic Claude models",
		URL:         "https://anthropic.com",
		DocsURL:     "https://docs.anthropic.com",
	},
	ai.ProviderOpenAI: {
		Name:        "OpenAI",
		ShortName:   "OpenAI",
		Icon:        "◆",
		Description: "OpenAI GPT models",
		URL:         "https://openai.com",
		DocsURL:     "https://platform.openai.com/docs",
	},
	ai.ProviderGoogle: {
		Name:        "Google",
		ShortName:   "Google",
		Icon:        "◇",
		Description: "Google Gemini models",
		URL:         "https://ai.google.dev",
		DocsURL:     "https://ai.google.dev/docs",
	},
	ai.ProviderGoogleVertex: {
		Name:        "Vertex AI",
		ShortName:   "Vertex",
		Icon:        "◇",
		Description: "Google Vertex AI models",
		URL:         "https://cloud.google.com/vertex-ai",
		DocsURL:     "https://cloud.google.com/vertex-ai/docs",
	},
	ai.ProviderDeepSeek: {
		Name:        "DeepSeek",
		ShortName:   "DeepSeek",
		Icon:        "◈",
		Description: "DeepSeek models",
		URL:         "https://deepseek.com",
		DocsURL:     "https://platform.deepseek.com/docs",
	},
	ai.ProviderMistral: {
		Name:        "Mistral",
		ShortName:   "Mistral",
		Icon:        "◉",
		Description: "Mistral AI models",
		URL:         "https://mistral.ai",
		DocsURL:     "https://docs.mistral.ai",
	},
	ai.ProviderGroq: {
		Name:        "Groq",
		ShortName:   "Groq",
		Icon:        "▣",
		Description: "Groq LPU inference",
		URL:         "https://groq.com",
		DocsURL:     "https://console.groq.com/docs",
	},
	ai.ProviderFireworks: {
		Name:        "Fireworks",
		ShortName:   "Fireworks",
		Icon:        "▤",
		Description: "Fireworks AI models",
		URL:         "https://fireworks.ai",
		DocsURL:     "https://docs.fireworks.ai",
	},
	ai.ProviderTogether: {
		Name:        "Together",
		ShortName:   "Together",
		Icon:        "▥",
		Description: "Together AI models",
		URL:         "https://together.ai",
		DocsURL:     "https://docs.together.ai",
	},
	ai.ProviderXAI: {
		Name:        "xAI",
		ShortName:   "xAI",
		Icon:        "✕",
		Description: "xAI Grok models",
		URL:         "https://x.ai",
		DocsURL:     "https://docs.x.ai",
	},
	ai.ProviderOpenRouter: {
		Name:        "OpenRouter",
		ShortName:   "ORouter",
		Icon:        "◎",
		Description: "OpenRouter unified API",
		URL:         "https://openrouter.ai",
		DocsURL:     "https://openrouter.ai/docs",
	},
	ai.ProviderVercelAIGateway: {
		Name:        "Vercel AI Gateway",
		ShortName:   "Vercel",
		Icon:        "▲",
		Description: "Vercel AI Gateway",
		URL:         "https://vercel.com",
		DocsURL:     "https://vercel.com/docs/ai",
	},
	ai.ProviderAmazonBedrock: {
		Name:        "Bedrock",
		ShortName:   "Bedrock",
		Icon:        "▦",
		Description: "AWS Bedrock models",
		URL:         "https://aws.amazon.com/bedrock",
		DocsURL:     "https://docs.aws.amazon.com/bedrock",
	},
	ai.ProviderGitHubCopilot: {
		Name:        "GitHub Copilot",
		ShortName:   "Copilot",
		Icon:        "◆",
		Description: "GitHub Copilot models",
		URL:         "https://github.com/features/copilot",
		DocsURL:     "https://docs.github.com/copilot",
	},
	ai.ProviderCerebras: {
		Name:        "Cerebras",
		ShortName:   "Cerebras",
		Icon:        "▧",
		Description: "Cerebras inference",
		URL:         "https://cerebras.ai",
		DocsURL:     "https://inference.cerebras.ai/docs",
	},
	ai.ProviderAzureOpenAIResponses: {
		Name:        "Azure OpenAI",
		ShortName:   "Azure",
		Icon:        "▨",
		Description: "Azure OpenAI Service",
		URL:         "https://azure.microsoft.com/products/ai-services",
		DocsURL:     "https://learn.microsoft.com/azure/ai-services",
	},
	ai.ProviderOpenAICodex: {
		Name:        "OpenAI Codex",
		ShortName:   "Codex",
		Icon:        "◆",
		Description: "OpenAI Codex models",
		URL:         "https://openai.com",
		DocsURL:     "https://platform.openai.com/docs",
	},
}

// GetProviderDisplayName returns the human-readable display name for a provider.
func GetProviderDisplayName(provider ai.Provider) string {
	if info, ok := providerDisplayNames[provider]; ok {
		return info.Name
	}
	return string(provider)
}

// GetProviderShortName returns the short display name.
func GetProviderShortName(provider ai.Provider) string {
	if info, ok := providerDisplayNames[provider]; ok {
		return info.ShortName
	}
	return string(provider)
}

// GetProviderIcon returns a text icon for a provider.
func GetProviderIcon(provider ai.Provider) string {
	if info, ok := providerDisplayNames[provider]; ok {
		return info.Icon
	}
	return "?"
}

// GetProviderDisplayInfo returns the full display info for a provider.
func GetProviderDisplayInfo(provider ai.Provider) ProviderDisplayInfo {
	if info, ok := providerDisplayNames[provider]; ok {
		return info
	}
	return ProviderDisplayInfo{
		Name:      string(provider),
		ShortName: string(provider),
		Icon:      "?",
	}
}

// FormatProviderWithIcon formats a provider name with its icon for display.
func FormatProviderWithIcon(provider ai.Provider) string {
	info := GetProviderDisplayInfo(provider)
	return info.Icon + " " + info.Name
}

// AllProviderDisplayInfos returns all registered provider display info.
func AllProviderDisplayInfos() []ProviderDisplayInfo {
	var infos []ProviderDisplayInfo
	for _, info := range providerDisplayNames {
		infos = append(infos, info)
	}
	return infos
}
