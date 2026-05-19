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

const mistralDefaultBaseURL = "https://api.mistral.ai"

// MistralOptions extends base options.
type MistralOptions struct {
	ai.StreamOptions
	ToolChoice interface{} `json:"tool_choice,omitempty"`
	SafePrompt *bool       `json:"safe_prompt,omitempty"`
}

// StreamMistral streams against the Mistral Chat API.
func StreamMistral(model ai.Model, ctx ai.Context, options *ai.StreamOptions, callback ai.StreamEventCallback) error {
	opts := &MistralOptions{}
	if options != nil {
		opts.StreamOptions = *options
	}
	return streamMistral(model, ctx, opts, callback)
}

// StreamSimpleMistral is the simple version.
func StreamSimpleMistral(model ai.Model, ctx ai.Context, options *ai.SimpleStreamOptions, callback ai.StreamEventCallback) error {
	opts := &MistralOptions{}
	if options != nil {
		opts.StreamOptions = options.StreamOptions
	}
	return streamMistral(model, ctx, opts, callback)
}

func streamMistral(model ai.Model, ctx ai.Context, opts *MistralOptions, callback ai.StreamEventCallback) error {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = GetEnvAPIKey("MISTRAL_API_KEY")
	}
	if apiKey == "" {
		return fmt.Errorf("MISTRAL_API_KEY not set")
	}

	baseURL := model.BaseURL
	if baseURL == "" {
		baseURL = BaseURLFromEnv("MISTRAL_BASE_URL", mistralDefaultBaseURL)
	}

	body := map[string]interface{}{
		"model":    model.Name,
		"stream":   true,
		"messages": buildMistralMessages(ctx),
	}
	if opts.MaxTokens > 0 {
		body["max_tokens"] = opts.MaxTokens
	}
	if opts.Temperature > 0 {
		body["temperature"] = opts.Temperature
	}
	if opts.SafePrompt != nil {
		body["safe_prompt"] = *opts.SafePrompt
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
	if opts.ToolChoice != nil {
		body["tool_choice"] = opts.ToolChoice
	}

	jsonData, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", baseURL+"/v1/chat/completions", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Mistral request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Mistral API error (status %d): %s", resp.StatusCode, string(b))
	}

	return parseMistralStream(resp.Body, callback, model)
}

func buildMistralMessages(ctx ai.Context) []map[string]interface{} {
	var messages []map[string]interface{}
	if ctx.SystemPrompt != "" {
		messages = append(messages, map[string]interface{}{
			"role":    "system",
			"content": ctx.SystemPrompt,
		})
	}
	for _, msg := range ctx.Messages {
		switch {
		case msg.User != nil:
			messages = append(messages, map[string]interface{}{
				"role":    "user",
				"content": msg.User.Content,
			})
		case msg.Assistant != nil:
			m := map[string]interface{}{"role": "assistant"}
			var contentParts []map[string]interface{}
			for _, block := range msg.Assistant.Content {
				if block.Text != nil {
					contentParts = append(contentParts, map[string]interface{}{"type": "text", "text": block.Text.Text})
				}
			}
			if len(contentParts) > 0 {
				m["content"] = contentParts
			} else {
				m["content"] = ""
			}
			for _, block := range msg.Assistant.Content {
				if block.ToolCall != nil {
					argsJSON, _ := json.Marshal(block.ToolCall.Arguments)
					m["tool_calls"] = append(m["tool_calls"].([]interface{}), map[string]interface{}{
						"id":   block.ToolCall.ID,
						"type": "function",
						"function": map[string]interface{}{
							"name":      block.ToolCall.Name,
							"arguments": string(argsJSON),
						},
					})
				}
			}
			messages = append(messages, m)
		case msg.ToolResult != nil:
			tr := msg.ToolResult
			content := ""
			if len(tr.Content) > 0 && tr.Content[0].Text != nil {
				content = tr.Content[0].Text.Text
			}
			messages = append(messages, map[string]interface{}{
				"role":        "tool",
				"tool_call_id": tr.ToolCallID,
				"content":     content,
			})
		}
	}
	return messages
}

type mistralChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		Delta        struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

func parseMistralStream(body io.ReadCloser, callback ai.StreamEventCallback, model ai.Model) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var msg ai.AssistantMessage
	msg.Role = ai.RoleAssistant
	msg.API = model.API
	msg.Provider = model.Provider
	msg.Model = model.Name
	msg.Timestamp = time.Now().UnixMilli()

	var contentBuilder strings.Builder
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

		var chunk mistralChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.ID != "" {
			msg.ResponseID = chunk.ID
		}
		if chunk.Model != "" {
			msg.ResponseModel = chunk.Model
		}
		if chunk.Usage != nil {
			msg.Usage.Input = chunk.Usage.PromptTokens
			msg.Usage.Output = chunk.Usage.CompletionTokens
			msg.Usage.TotalTokens = chunk.Usage.TotalTokens
		}

		for _, choice := range chunk.Choices {
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
			for _, tc := range choice.Delta.ToolCalls {
				b, ok := toolCallBuilders[tc.Index]
				if !ok {
					b = &struct {
						id   string
						name string
						args strings.Builder
					}{}
					toolCallBuilders[tc.Index] = b
				}
				if tc.ID != "" {
					b.id = tc.ID
				}
				if tc.Function.Name != "" {
					b.name = tc.Function.Name
				}
				b.args.WriteString(tc.Function.Arguments)
			}
			if choice.FinishReason != nil {
				switch *choice.FinishReason {
				case "stop":
					msg.StopReason = ai.StopReasonStop
				case "length":
					msg.StopReason = ai.StopReasonLength
				case "tool_calls":
					msg.StopReason = ai.StopReasonToolUse
				}
			}
		}
	}

	for _, b := range toolCallBuilders {
		var args map[string]interface{}
		json.Unmarshal([]byte(b.args.String()), &args)
		msg.Content = append(msg.Content, ai.ContentBlock{
			ToolCall: &ai.ToolCall{
				Type:      "toolCall",
				ID:        b.id,
				Name:      b.name,
				Arguments: args,
			},
		})
	}
	if msg.StopReason == "" {
		msg.StopReason = ai.StopReasonStop
	}
	callback(ai.StreamEvent{Type: "done", Message: &msg})
	return scanner.Err()
}

func init() {
	Register(&StreamProvider{
		API:          ai.APIMistralConversations,
		Stream:       StreamMistral,
		StreamSimple: StreamSimpleMistral,
	})
}
