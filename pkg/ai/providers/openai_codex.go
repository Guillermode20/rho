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

const codexDefaultBaseURL = "https://api.openai.com"

// CodexOptions extends base options.
type CodexOptions struct {
	ai.StreamOptions
	ToolChoice interface{} `json:"tool_choice,omitempty"`
}

// StreamOpenAICodex streams against the OpenAI Codex Responses API.
func StreamOpenAICodex(model ai.Model, ctx ai.Context, options *ai.StreamOptions, callback ai.StreamEventCallback) error {
	opts := &CodexOptions{}
	if options != nil {
		opts.StreamOptions = *options
	}
	return streamCodex(model, ctx, opts, callback)
}

// StreamSimpleOpenAICodex is the simple version.
func StreamSimpleOpenAICodex(model ai.Model, ctx ai.Context, options *ai.SimpleStreamOptions, callback ai.StreamEventCallback) error {
	opts := &CodexOptions{}
	if options != nil {
		opts.StreamOptions = options.StreamOptions
	}
	return streamCodex(model, ctx, opts, callback)
}

func streamCodex(model ai.Model, ctx ai.Context, opts *CodexOptions, callback ai.StreamEventCallback) error {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = GetEnvAPIKey("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return fmt.Errorf("OPENAI_API_KEY not set")
	}

	baseURL := model.BaseURL
	if baseURL == "" {
		baseURL = BaseURLFromEnv("OPENAI_BASE_URL", codexDefaultBaseURL)
	}

	body := map[string]interface{}{
		"model":    model.Name,
		"messages": buildCodexMessages(ctx),
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
	req, _ := http.NewRequestWithContext(context.Background(), "POST", baseURL+"/v1/chat/completions", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Codex request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Codex API error (status %d): %s", resp.StatusCode, string(b))
	}

	return parseCodexStream(resp.Body, callback, model)
}

func buildCodexMessages(ctx ai.Context) []map[string]interface{} {
	return buildOpenAIMessages(ctx)
}

type codexStreamChunk struct {
	ID      string `json:"id"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			Reasoning string `json:"reasoning_content"`
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
}

func parseCodexStream(body io.ReadCloser, callback ai.StreamEventCallback, model ai.Model) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var msg ai.AssistantMessage
	msg.Role = ai.RoleAssistant
	msg.API = model.API
	msg.Provider = model.Provider
	msg.Model = model.Name
	msg.Timestamp = time.Now().UnixMilli()

	var contentBuilder, reasoningBuilder strings.Builder
	toolCallBuilders := make(map[int]*struct{ id, name string; args strings.Builder })

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk codexStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.ID != "" {
			msg.ResponseID = chunk.ID
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				contentBuilder.WriteString(choice.Delta.Content)
				msg.Content = []ai.ContentBlock{{Text: &ai.TextContent{Type: "text", Text: contentBuilder.String()}}}
				callback(ai.StreamEvent{Type: "text_delta", Delta: choice.Delta.Content})
			}
			if choice.Delta.Reasoning != "" {
				reasoningBuilder.WriteString(choice.Delta.Reasoning)
				callback(ai.StreamEvent{Type: "thinking_delta", Delta: choice.Delta.Reasoning})
			}
			for _, tc := range choice.Delta.ToolCalls {
				b, ok := toolCallBuilders[tc.Index]
				if !ok {
					b = &struct{ id, name string; args strings.Builder }{}
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
			ToolCall: &ai.ToolCall{Type: "toolCall", ID: b.id, Name: b.name, Arguments: args},
		})
	}
	if reasoningBuilder.Len() > 0 {
		msg.Content = append([]ai.ContentBlock{{Thinking: &ai.ThinkingContent{Type: "thinking", Thinking: reasoningBuilder.String()}}}, msg.Content...)
	}
	if msg.StopReason == "" {
		msg.StopReason = ai.StopReasonStop
	}
	callback(ai.StreamEvent{Type: "done", Message: &msg})
	return scanner.Err()
}

func init() {
	Register(&StreamProvider{
		API:          ai.APIOpenAICodexResponses,
		Stream:       StreamOpenAICodex,
		StreamSimple: StreamSimpleOpenAICodex,
	})
}
