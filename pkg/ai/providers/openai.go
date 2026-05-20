package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/earendil-works/rho/pkg/ai"
)

const (
	openaiDefaultBaseURL    = "https://api.openai.com"
	openaiDefaultAPIVersion = "2024-10-01"
)

// OpenAICompletionsOptions extends base options for completions.
type OpenAICompletionsOptions struct {
	ai.StreamOptions
	ReasoningEffort string      `json:"reasoning_effort,omitempty"`
	ToolChoice      interface{} `json:"tool_choice,omitempty"`
	Store           *bool       `json:"store,omitempty"`
	User            string      `json:"user,omitempty"`
}

// StreamOpenAICompletions streams against the OpenAI Chat Completions API.
func StreamOpenAICompletions(model ai.Model, ctx ai.Context, options *ai.StreamOptions, callback ai.StreamEventCallback) error {
	opts := &OpenAICompletionsOptions{}
	if options != nil {
		opts.StreamOptions = *options
	}
	return streamOpenAICompletions(model, ctx, opts, callback)
}

// StreamSimpleOpenAICompletions is the simple version.
func StreamSimpleOpenAICompletions(model ai.Model, ctx ai.Context, options *ai.SimpleStreamOptions, callback ai.StreamEventCallback) error {
	opts := &OpenAICompletionsOptions{}
	if options != nil {
		opts.StreamOptions = options.StreamOptions
		if options.Reasoning != "" {
			opts.ReasoningEffort = string(options.Reasoning)
		}
	}
	return streamOpenAICompletions(model, ctx, opts, callback)
}

func streamOpenAICompletions(model ai.Model, ctx ai.Context, opts *OpenAICompletionsOptions, callback ai.StreamEventCallback) error {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = openAICompatibleAPIKey(model.Provider)
	}
	if apiKey == "" {
		return fmt.Errorf("%s API key not set", model.Provider)
	}

	baseURL := model.BaseURL
	if baseURL == "" {
		baseURL = openAICompatibleBaseURL(model.Provider)
	}

	body := buildOpenAICompletionsRequest(model, ctx, opts)
	jsonData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	reqCtx := context.Background()
	if opts.Signal != nil {
		reqCtx = opts.Signal
	}
	req, err := http.NewRequestWithContext(reqCtx, "POST", openAIChatCompletionsURL(baseURL), bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	if opts.TimeoutMs > 0 {
		client := &http.Client{Timeout: time.Duration(opts.TimeoutMs) * time.Millisecond}
		return sendOpenAIStreamingRequest(client, req, callback, model)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	return sendOpenAIStreamingRequest(client, req, callback, model)
}

func openAICompatibleAPIKey(provider ai.Provider) string {
	switch provider {
	case ai.ProviderCrof:
		return GetEnvAPIKey("CROF_API_KEY", "CROFAI_API_KEY")
	case ai.ProviderDeepSeek:
		return GetEnvAPIKey("DEEPSEEK_API_KEY")
	default:
		return GetEnvAPIKey("OPENAI_API_KEY")
	}
}

func openAICompatibleBaseURL(provider ai.Provider) string {
	switch provider {
	case ai.ProviderCrof:
		return BaseURLFromEnv("CROF_BASE_URL", "https://crof.ai")
	case ai.ProviderDeepSeek:
		return BaseURLFromEnv("DEEPSEEK_BASE_URL", "https://api.deepseek.com")
	default:
		return BaseURLFromEnv("OPENAI_BASE_URL", openaiDefaultBaseURL)
	}
}

func openAIChatCompletionsURL(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(baseURL, "/v1") || strings.HasSuffix(baseURL, "/v2") {
		return baseURL + "/chat/completions"
	}
	return baseURL + "/v1/chat/completions"
}

func buildOpenAICompletionsRequest(model ai.Model, ctx ai.Context, opts *OpenAICompletionsOptions) map[string]interface{} {
	body := map[string]interface{}{
		"model":    model.Name,
		"stream":   true,
		"messages": buildOpenAIMessages(ctx),
	}

	if opts.MaxTokens > 0 {
		body["max_tokens"] = opts.MaxTokens
	}
	if opts.Temperature > 0 {
		body["temperature"] = opts.Temperature
	}
	if opts.ReasoningEffort != "" {
		body["reasoning_effort"] = opts.ReasoningEffort
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
		body["tool_choice"] = "auto"
	}
	if opts.ToolChoice != nil {
		body["tool_choice"] = opts.ToolChoice
	}

	return body
}

func buildOpenAIMessages(ctx ai.Context) []map[string]interface{} {
	var messages []map[string]interface{}

	// System prompt
	if ctx.SystemPrompt != "" {
		messages = append(messages, map[string]interface{}{
			"role":    "system",
			"content": ctx.SystemPrompt,
		})
	}

	for _, msg := range ctx.Messages {
		switch {
		case msg.User != nil:
			m := map[string]interface{}{
				"role":    "user",
				"content": msg.User.Content,
			}
			messages = append(messages, m)

		case msg.Assistant != nil:
			m := map[string]interface{}{
				"role": "assistant",
			}
			var contentParts []map[string]interface{}
			for _, block := range msg.Assistant.Content {
				switch {
				case block.Text != nil:
					contentParts = append(contentParts, map[string]interface{}{
						"type": "text",
						"text": block.Text.Text,
					})
				case block.ToolCall != nil:
					argsJSON, _ := json.Marshal(block.ToolCall.Arguments)
					toolCalls, _ := m["tool_calls"].([]interface{})
					toolCalls = append(toolCalls, map[string]interface{}{
						"id":   block.ToolCall.ID,
						"type": "function",
						"function": map[string]interface{}{
							"name":      block.ToolCall.Name,
							"arguments": string(argsJSON),
						},
					})
					m["tool_calls"] = toolCalls
				}
			}
			if len(contentParts) > 0 {
				m["content"] = contentParts
			} else if m["tool_calls"] == nil {
				m["content"] = ""
			}
			messages = append(messages, m)

		case msg.ToolResult != nil:
			tr := msg.ToolResult
			content := ""
			if len(tr.Content) > 0 && tr.Content[0].Text != nil {
				content = tr.Content[0].Text.Text
			}
			messages = append(messages, map[string]interface{}{
				"role":         "tool",
				"tool_call_id": tr.ToolCallID,
				"content":      content,
			})
		}
	}
	return messages
}

func sendOpenAIStreamingRequest(client *http.Client, req *http.Request, callback ai.StreamEventCallback, model ai.Model) error {
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return parseOpenAIStream(resp.Body, callback, model)
}

// OpenAI SSE response types
type openAIStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Delta        openAIDelta `json:"delta"`
		FinishReason *string     `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

type openAIDelta struct {
	Role      string `json:"role,omitempty"`
	Content   string `json:"content,omitempty"`
	Reasoning string `json:"reasoning_content,omitempty"`
	ToolCalls []struct {
		Index    int    `json:"index"`
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	} `json:"tool_calls,omitempty"`
}

func parseOpenAIStream(body io.ReadCloser, callback ai.StreamEventCallback, model ai.Model) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var msg ai.AssistantMessage
	msg.Role = ai.RoleAssistant
	msg.API = model.API
	msg.Provider = model.Provider
	msg.Model = model.Name
	msg.Timestamp = time.Now().UnixMilli()

	var contentBuilder strings.Builder
	var reasoningBuilder strings.Builder
	toolCallBuilders := make(map[int]*struct {
		id   string
		name string
		args strings.Builder
	})

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// Track response ID and model
		if chunk.ID != "" {
			msg.ResponseID = chunk.ID
		}
		if chunk.Model != "" {
			msg.ResponseModel = chunk.Model
		}

		// Track usage
		if chunk.Usage != nil {
			msg.Usage.Input = chunk.Usage.PromptTokens
			msg.Usage.Output = chunk.Usage.CompletionTokens
			msg.Usage.TotalTokens = chunk.Usage.TotalTokens
		}

		for _, choice := range chunk.Choices {
			// Text content
			if choice.Delta.Content != "" {
				contentBuilder.WriteString(choice.Delta.Content)
				msg.Content = []ai.ContentBlock{
					{Text: &ai.TextContent{Type: "text", Text: contentBuilder.String()}},
				}
				callback(ai.StreamEvent{
					Type:  "text_delta",
					Delta: choice.Delta.Content,
					Partial: &ai.AssistantMessage{
						Role:    ai.RoleAssistant,
						Content: msg.Content,
					},
				})
			}

			// Reasoning content
			if choice.Delta.Reasoning != "" {
				reasoningBuilder.WriteString(choice.Delta.Reasoning)
				callback(ai.StreamEvent{
					Type:  "thinking_delta",
					Delta: choice.Delta.Reasoning,
				})
			}

			// Tool calls
			for _, tc := range choice.Delta.ToolCalls {
				b, ok := toolCallBuilders[tc.Index]
				if !ok {
					b = &struct {
						id   string
						name string
						args strings.Builder
					}{}
					toolCallBuilders[tc.Index] = b
					callback(ai.StreamEvent{
						Type:         "toolcall_start",
						ContentIndex: tc.Index,
						ToolCall: &ai.ToolCall{
							Type: "toolCall",
						},
					})
				}
				if tc.ID != "" {
					b.id = tc.ID
				}
				if tc.Function.Name != "" {
					b.name = tc.Function.Name
				}
				b.args.WriteString(tc.Function.Arguments)
				callback(ai.StreamEvent{
					Type:         "toolcall_delta",
					ContentIndex: tc.Index,
					Delta:        tc.Function.Arguments,
					ToolCall: &ai.ToolCall{
						Type:      "toolCall",
						ID:        b.id,
						Name:      b.name,
						Arguments: parseToolArguments(b.args.String()),
					},
				})
			}

			// Finish reason
			if choice.FinishReason != nil {
				switch *choice.FinishReason {
				case "stop":
					msg.StopReason = ai.StopReasonStop
				case "length":
					msg.StopReason = ai.StopReasonLength
				case "tool_calls":
					msg.StopReason = ai.StopReasonToolUse
				default:
					msg.StopReason = ai.StopReasonStop
				}
			}
		}
	}

	// Build tool calls from collected chunks
	for _, b := range toolCallBuilders {
		tc := ai.ToolCall{
			Type:      "toolCall",
			ID:        b.id,
			Name:      b.name,
			Arguments: parseToolArguments(b.args.String()),
		}
		msg.Content = append(msg.Content, ai.ContentBlock{ToolCall: &tc})
		callback(ai.StreamEvent{
			Type:     "toolcall_end",
			ToolCall: &tc,
		})
	}

	if msg.StopReason == "" {
		msg.StopReason = ai.StopReasonStop
	}

	// If we have reasoning content, add it as a thinking block
	if reasoningBuilder.Len() > 0 {
		msg.Content = append([]ai.ContentBlock{
			{Thinking: &ai.ThinkingContent{Type: "thinking", Thinking: reasoningBuilder.String()}},
		}, msg.Content...)
	}

	callback(ai.StreamEvent{
		Type:    "done",
		Message: &msg,
	})

	return scanner.Err()
}

func parseToolArguments(raw string) map[string]interface{} {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil
	}
	return args
}

// OpenAI Responses API

// StreamOpenAIResponses streams against the OpenAI Responses API.
func StreamOpenAIResponses(model ai.Model, ctx ai.Context, options *ai.StreamOptions, callback ai.StreamEventCallback) error {
	// The Responses API uses a similar streaming format but different endpoint
	apiKey := options.APIKey
	if apiKey == "" {
		apiKey = GetEnvAPIKey("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return fmt.Errorf("OPENAI_API_KEY not set")
	}

	baseURL := model.BaseURL
	if baseURL == "" {
		baseURL = BaseURLFromEnv("OPENAI_BASE_URL", openaiDefaultBaseURL)
	}

	body := map[string]interface{}{
		"model":  model.Name,
		"stream": true,
		"input":  buildOpenAIMessages(ctx),
		"tools":  buildOpenAITools(ctx),
	}
	if options.MaxTokens > 0 {
		body["max_output_tokens"] = options.MaxTokens
	}
	if options.Temperature > 0 {
		body["temperature"] = options.Temperature
	}

	// Add instructions (system prompt)
	if ctx.SystemPrompt != "" {
		body["instructions"] = ctx.SystemPrompt
	}

	jsonData, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", baseURL+"/v1/responses", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OpenAI Responses API error (status %d): %s", resp.StatusCode, string(b))
	}

	return parseOpenAIStream(resp.Body, callback, model)
}

// StreamSimpleOpenAIResponses is the simple version.
func StreamSimpleOpenAIResponses(model ai.Model, ctx ai.Context, options *ai.SimpleStreamOptions, callback ai.StreamEventCallback) error {
	opts := &ai.StreamOptions{}
	if options != nil {
		opts = &options.StreamOptions
	}
	return StreamOpenAIResponses(model, ctx, opts, callback)
}

func buildOpenAITools(ctx ai.Context) []map[string]interface{} {
	if len(ctx.Tools) == 0 {
		return nil
	}
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
	return tools
}

func init() {
	Register(&StreamProvider{
		API:          ai.APIOpenAICompletions,
		Stream:       StreamOpenAICompletions,
		StreamSimple: StreamSimpleOpenAICompletions,
	})
	Register(&StreamProvider{
		API:          ai.APIOpenAIResponses,
		Stream:       StreamOpenAIResponses,
		StreamSimple: StreamSimpleOpenAIResponses,
	})
}
