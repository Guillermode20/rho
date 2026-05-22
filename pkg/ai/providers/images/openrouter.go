// Package imagesproviders implements image generation providers.
package imagesproviders

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/earendil-works/rho/pkg/ai"
	"github.com/earendil-works/rho/pkg/ai/providers"
)

const openRouterImageBaseURL = "https://openrouter.ai/api/v1"

func init() {
	ai.RegisterImagesProvider(ai.ImagesAPIOpenRouter, StreamOpenRouterImages)
}

// StreamOpenRouterImages generates images via OpenRouter's API.
func StreamOpenRouterImages(model ai.ImageModel, ctx ai.ImagesContext, options *ai.ImagesOptions) (*ai.AssistantImages, error) {
	apiKey := ""
	if options != nil && options.APIKey != "" {
		apiKey = options.APIKey
	}
	if apiKey == "" {
		apiKey = providers.GetEnvAPIKey("OPENROUTER_API_KEY", "OPENAI_API_KEY")
	}
	if apiKey == "" {
		return errorResult(model, "OPENROUTER_API_KEY or OPENAI_API_KEY not set"), nil
	}

	baseURL := model.BaseURL
	if baseURL == "" {
		baseURL = providers.BaseURLFromEnv("OPENROUTER_BASE_URL", openRouterImageBaseURL)
	}

	// Build prompt from input
	prompt := ""
	for _, input := range ctx.Input {
		if input.Type == "text" {
			if prompt != "" {
				prompt += "\n"
			}
			prompt += input.Text
		}
	}

	if prompt == "" {
		return errorResult(model, "no text prompt provided"), nil
	}

	// Determine size
	size := "1024x1024"
	n := 1
	if options != nil {
		if options.Size != "" {
			size = options.Size
		}
		if options.N > 0 {
			n = options.N
		}
	}

	// Try DALL-E 3 via OpenAI-compatible endpoint first, then fall back to
	// OpenRouter's /api/v1/image/completions or model-specific routing.
	if strings.Contains(model.Name, "dall-e") {
		return callOpenAIImageAPI(model, prompt, n, size, apiKey, options)
	}

	// For other models, use OpenRouter's chat completions with image generation
	return callOpenRouterImageGen(model, prompt, n, apiKey, baseURL, options)
}

func callOpenAIImageAPI(model ai.ImageModel, prompt string, n int, size, apiKey string, opts *ai.ImagesOptions) (*ai.AssistantImages, error) {
	quality := "standard"
	style := "vivid"
	if opts != nil {
		if opts.Quality != "" {
			quality = opts.Quality
		}
		if opts.Style != "" {
			style = opts.Style
		}
	}

	body := map[string]interface{}{
		"model":           model.Name,
		"prompt":          prompt,
		"n":               n,
		"size":            size,
		"quality":         quality,
		"style":           style,
		"response_format": "b64_json",
	}

	jsonData, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(context.Background(), "POST", "https://api.openai.com/v1/images/generations", bytes.NewReader(jsonData))
	if err != nil {
		return errorResult(model, fmt.Sprintf("request creation failed: %v", err)), nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return errorResult(model, fmt.Sprintf("API request failed: %v", err)), nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return errorResult(model, fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(respBody))), nil
	}

	var result struct {
		Created int64 `json:"created"`
		Data    []struct {
			B64JSON       string `json:"b64_json"`
			RevisedPrompt string `json:"revised_prompt,omitempty"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return errorResult(model, fmt.Sprintf("response parse failed: %v", err)), nil
	}

	var output []ai.ImagesOutputContent
	for _, d := range result.Data {
		output = append(output, ai.ImagesOutputContent{
			Type:     "image",
			Data:     d.B64JSON,
			MimeType: "image/png",
			Text:     d.RevisedPrompt,
		})
	}

	return &ai.AssistantImages{
		API:        model.API,
		Provider:   model.Provider,
		Model:      model.Name,
		Output:     output,
		StopReason: ai.ImagesStopStop,
		Timestamp:  time.Now().UnixMilli(),
	}, nil
}

func callOpenRouterImageGen(model ai.ImageModel, prompt string, n int, apiKey, baseURL string, opts *ai.ImagesOptions) (*ai.AssistantImages, error) {
	body := map[string]interface{}{
		"model": model.Name,
		"messages": []map[string]interface{}{
			{"role": "user", "content": prompt},
		},
	}

	jsonData, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(context.Background(), "POST", baseURL+"/chat/completions", bytes.NewReader(jsonData))
	if err != nil {
		return errorResult(model, fmt.Sprintf("request failed: %v", err)), nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", "https://github.com/earendil-works/rho")
	req.Header.Set("X-Title", "rho")

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return errorResult(model, fmt.Sprintf("API request failed: %v", err)), nil
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return errorResult(model, fmt.Sprintf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))), nil
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return errorResult(model, fmt.Sprintf("response parse failed: %v", err)), nil
	}

	output := []ai.ImagesOutputContent{{
		Type: "text",
		Text: chatResp.Choices[0].Message.Content,
	}}

	return &ai.AssistantImages{
		API:        model.API,
		Provider:   model.Provider,
		Model:      model.Name,
		Output:     output,
		StopReason: ai.ImagesStopStop,
		Timestamp:  time.Now().UnixMilli(),
	}, nil
}

func errorResult(model ai.ImageModel, msg string) *ai.AssistantImages {
	return &ai.AssistantImages{
		API:          model.API,
		Provider:     model.Provider,
		Model:        model.Name,
		StopReason:   ai.ImagesStopError,
		ErrorMessage: msg,
		Timestamp:    time.Now().UnixMilli(),
	}
}
