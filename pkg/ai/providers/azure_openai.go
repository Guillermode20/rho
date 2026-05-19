package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/earendil-works/rho/pkg/ai"
)

// AzureOpenAIOptions extends base options.
type AzureOpenAIOptions struct {
	ai.StreamOptions
	ReasoningEffort string      `json:"reasoning_effort,omitempty"`
	ToolChoice      interface{} `json:"tool_choice,omitempty"`
}

// StreamAzureOpenAI streams against the Azure OpenAI Chat Completions API.
func StreamAzureOpenAI(model ai.Model, ctx ai.Context, options *ai.StreamOptions, callback ai.StreamEventCallback) error {
	opts := &AzureOpenAIOptions{}
	if options != nil {
		opts.StreamOptions = *options
	}
	return streamAzureOpenAI(model, ctx, opts, callback)
}

// StreamSimpleAzureOpenAI is the simple version.
func StreamSimpleAzureOpenAI(model ai.Model, ctx ai.Context, options *ai.SimpleStreamOptions, callback ai.StreamEventCallback) error {
	opts := &AzureOpenAIOptions{}
	if options != nil {
		opts.StreamOptions = options.StreamOptions
	}
	return streamAzureOpenAI(model, ctx, opts, callback)
}

func streamAzureOpenAI(model ai.Model, ctx ai.Context, opts *AzureOpenAIOptions, callback ai.StreamEventCallback) error {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = GetEnvAPIKey("AZURE_OPENAI_API_KEY")
	}
	if apiKey == "" {
		return fmt.Errorf("AZURE_OPENAI_API_KEY not set")
	}

	baseURL := model.BaseURL
	if baseURL == "" {
		baseURL = BaseURLFromEnv("AZURE_OPENAI_BASE_URL", "https://YOUR_RESOURCE.openai.azure.com")
	}

	// Azure OpenAI uses deployment names, not model names
	// The model.Name is the deployment ID
	deployment := model.Name
	apiVersion := "2024-10-01-preview"

	body := map[string]interface{}{
		"messages": buildAzureMessages(ctx),
		"stream":   true,
	}
	if opts.MaxTokens > 0 {
		body["max_tokens"] = opts.MaxTokens
	}
	if opts.Temperature > 0 {
		body["temperature"] = opts.Temperature
	}
	if len(ctx.Tools) > 0 {
		var tools []map[string]interface{}
		for _, t := range ctx.Tools {
			tools = append(tools, map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  t.Parameters,
				},
			})
		}
		body["tools"] = tools
	}

	jsonData, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s", baseURL, deployment, apiVersion)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", apiKey)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Azure OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Azure OpenAI API error (status %d): %s", resp.StatusCode, string(b))
	}

	return parseOpenAIStream(resp.Body, callback, model)
}

func buildAzureMessages(ctx ai.Context) []map[string]interface{} {
	return buildOpenAIMessages(ctx)
}

func init() {
	Register(&StreamProvider{
		API:          ai.APIAzureOpenAIResponses,
		Stream:       StreamAzureOpenAI,
		StreamSimple: StreamSimpleAzureOpenAI,
	})
}
