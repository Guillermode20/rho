package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/earendil-works/rho/pkg/ai"
)

// ResolveBaseURL resolves the base URL for the given provider.
func ResolveBaseURL(provider ai.Provider) string {
	// 1. Check universal provider-specific env var override: e.g. CROF_BASE_URL, DEEPSEEK_BASE_URL, GROQ_BASE_URL
	envKey := strings.ToUpper(string(provider)) + "_BASE_URL"
	if val := os.Getenv(envKey); val != "" {
		return val
	}

	// 2. Switch for specific providers with known env vars (or custom logic)
	switch provider {
	case ai.ProviderAnthropic:
		return BaseURLFromEnv("ANTHROPIC_BASE_URL", "https://api.anthropic.com")
	case ai.ProviderGoogle:
		return BaseURLFromEnv("GOOGLE_GENAI_BASE_URL", "https://generativelanguage.googleapis.com")
	case ai.ProviderMistral:
		return BaseURLFromEnv("MISTRAL_BASE_URL", "https://api.mistral.ai")
	}

	// 3. Check built-in models first for a hardcoded BaseURL
	for _, m := range ai.DefaultModels() {
		if m.Provider == provider && m.BaseURL != "" {
			return m.BaseURL
		}
	}

	// 4. Fallback to OpenAI compatible base URL
	return openAICompatibleBaseURL(provider)
}

// FetchModelsForProvider queries the provider's API for the list of available models.
func FetchModelsForProvider(provider ai.Provider, apiKey string) ([]ai.ModelDefinition, error) {
	baseURL := ResolveBaseURL(provider)
	if baseURL == "" {
		return nil, fmt.Errorf("could not resolve base URL for provider %s", provider)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var req *http.Request
	var err error

	switch provider {
	case ai.ProviderGoogle:
		// Google models list API endpoint
		url := fmt.Sprintf("%s/v1beta/models?key=%s", strings.TrimRight(baseURL, "/"), apiKey)
		req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}

	case ai.ProviderAnthropic:
		// Anthropic models list API endpoint
		url := fmt.Sprintf("%s/v1/models", strings.TrimRight(baseURL, "/"))
		req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")

	default:
		// OpenAI compatible models list API endpoint
		url := strings.TrimRight(baseURL, "/")
		if strings.HasSuffix(url, "/v1") || strings.HasSuffix(url, "/v2") {
			url = url + "/models"
		} else {
			url = url + "/v1/models"
		}
		req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse responses
	switch provider {
	case ai.ProviderGoogle:
		var googleResp struct {
			Models []struct {
				Name                       string   `json:"name"`
				SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
			} `json:"models"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&googleResp); err != nil {
			return nil, fmt.Errorf("failed to decode Google response: %w", err)
		}

		var defs []ai.ModelDefinition
		for _, m := range googleResp.Models {
			// Only include generation models
			hasGen := false
			for _, method := range m.SupportedGenerationMethods {
				if method == "generateContent" {
					hasGen = true
					break
				}
			}
			if !hasGen {
				continue
			}
			name := strings.TrimPrefix(m.Name, "models/")
			defs = append(defs, GuessModelDefinition(provider, name))
		}
		return defs, nil

	case ai.ProviderAnthropic:
		var anthropicResp struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
			return nil, fmt.Errorf("failed to decode Anthropic response: %w", err)
		}

		var defs []ai.ModelDefinition
		for _, m := range anthropicResp.Data {
			defs = append(defs, GuessModelDefinition(provider, m.ID))
		}
		return defs, nil

	default:
		var openaiResp struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
			return nil, fmt.Errorf("failed to decode OpenAI-compatible response: %w", err)
		}

		var defs []ai.ModelDefinition
		for _, m := range openaiResp.Data {
			defs = append(defs, GuessModelDefinition(provider, m.ID))
		}
		return defs, nil
	}
}

// GuessModelDefinition fills in model definition metadata based on name and built-ins.
func GuessModelDefinition(provider ai.Provider, name string) ai.ModelDefinition {
	nameLower := strings.ToLower(name)

	// 1. Exact match (case-insensitive)
	for _, m := range ai.DefaultModels() {
		if m.Provider == provider && strings.ToLower(m.Name) == nameLower {
			return m
		}
	}

	// 2. Prefix/Contains match (case-insensitive)
	for _, m := range ai.DefaultModels() {
		mNameLower := strings.ToLower(m.Name)
		if m.Provider == provider && (strings.Contains(nameLower, mNameLower) || strings.Contains(mNameLower, nameLower)) {
			return m
		}
	}

	// Guess based on name
	reasoning := false
	if strings.Contains(nameLower, "reasoning") ||
		strings.Contains(nameLower, "thought") ||
		strings.Contains(nameLower, "deepseek-r1") ||
		strings.Contains(nameLower, "o1") ||
		strings.Contains(nameLower, "o3") ||
		strings.Contains(nameLower, "glm-5") ||
		strings.Contains(nameLower, "precision") ||
		strings.Contains(nameLower, "kimi-k2") {
		reasoning = true
	}

	input := []string{"text"}
	if strings.Contains(nameLower, "vision") ||
		strings.Contains(nameLower, "vl") ||
		strings.Contains(nameLower, "multimodal") ||
		strings.Contains(nameLower, "flash") ||
		strings.Contains(nameLower, "omni") ||
		strings.Contains(nameLower, "gpt-4") ||
		strings.Contains(nameLower, "sonnet") ||
		strings.Contains(nameLower, "opus") ||
		strings.Contains(nameLower, "gemini") ||
		strings.Contains(nameLower, "glm-4.7") ||
		strings.Contains(nameLower, "glm-5") ||
		strings.Contains(nameLower, "kimi") {
		input = append(input, "image")
	}

	// Guess context window and max tokens
	contextWindow := 128000
	maxTokens := 4096
	if strings.Contains(nameLower, "kimi") || strings.Contains(nameLower, "glm-5") || strings.Contains(nameLower, "qwen") || strings.Contains(nameLower, "mimo") {
		contextWindow = 262000
	} else if strings.Contains(nameLower, "deepseek") {
		contextWindow = 163000
	}

	var api ai.API
	switch provider {
	case ai.ProviderAnthropic:
		api = ai.APIAnthropicMessages
	case ai.ProviderGoogle:
		api = ai.APIGoogleGenerativeAI
	case ai.ProviderGoogleVertex:
		api = ai.APIGoogleVertex
	case ai.ProviderMistral:
		api = ai.APIMistralConversations
	case ai.ProviderAmazonBedrock:
		api = ai.APIBedrockConverseStream
	case ai.ProviderAzureOpenAIResponses:
		api = ai.APIAzureOpenAIResponses
	case ai.ProviderOpenAICodex:
		api = ai.APIOpenAICodexResponses
	default:
		api = ai.APIOpenAICompletions
	}

	return ai.ModelDefinition{
		API:           api,
		Provider:      provider,
		Name:          name,
		DisplayName:   name,
		ContextWindow: contextWindow,
		MaxTokens:     maxTokens,
		Reasoning:     reasoning,
		Input:         input,
		BaseURL:       ResolveBaseURL(provider),
	}
}
