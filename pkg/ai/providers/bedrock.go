package providers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/earendil-works/rho/pkg/ai"
)

// BedrockOptions extends base options.
type BedrockOptions struct {
	ai.StreamOptions
	Region          string `json:"region,omitempty"`
	AccessKeyID     string `json:"accessKeyId,omitempty"`
	SecretAccessKey string `json:"secretAccessKey,omitempty"`
	SessionToken    string `json:"sessionToken,omitempty"`
}

// StreamBedrock streams against Amazon Bedrock Converse Stream API.
func StreamBedrock(model ai.Model, ctx ai.Context, options *ai.StreamOptions, callback ai.StreamEventCallback) error {
	opts := &BedrockOptions{}
	if options != nil {
		opts.StreamOptions = *options
	}
	return fmt.Errorf("Amazon Bedrock provider requires AWS SDK credentials. Configure via environment variables AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION")
}

// StreamSimpleBedrock is the simple version.
func StreamSimpleBedrock(model ai.Model, ctx ai.Context, options *ai.SimpleStreamOptions, callback ai.StreamEventCallback) error {
	opts := &BedrockOptions{}
	if options != nil {
		opts.StreamOptions = options.StreamOptions
	}
	return fmt.Errorf("Amazon Bedrock provider requires AWS SDK credentials")
}

// bedrockSignRequest creates a signed request to Bedrock.
// In a full implementation, this would use AWS Signature V4 signing.
func bedrockSignRequest(method, url, region, service string, body []byte, accessKey, secretKey, sessionToken string) map[string]string {
	headers := map[string]string{
		"Content-Type": "application/json",
		"Host":         "bedrock-runtime." + region + ".amazonaws.com",
	}
	if sessionToken != "" {
		headers["X-Amz-Security-Token"] = sessionToken
	}
	return headers
}

// buildBedrockMessages converts our message format to Bedrock's format.
func buildBedrockMessages(ctx ai.Context) []map[string]interface{} {
	var messages []map[string]interface{}
	for _, msg := range ctx.Messages {
		switch {
		case msg.User != nil:
			messages = append(messages, map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{
					{"text": msg.User.Content},
				},
			})
		case msg.Assistant != nil:
			var content []map[string]interface{}
			for _, block := range msg.Assistant.Content {
				if block.Text != nil {
					content = append(content, map[string]interface{}{"text": block.Text.Text})
				}
				if block.ToolCall != nil {
					argsJSON, _ := json.Marshal(block.ToolCall.Arguments)
					content = append(content, map[string]interface{}{
						"toolUse": map[string]interface{}{
							"toolUseId": block.ToolCall.ID,
							"name":      block.ToolCall.Name,
							"input":     json.RawMessage(argsJSON),
						},
					})
				}
			}
			if len(content) == 0 {
				content = []map[string]interface{}{{"text": ""}}
			}
			messages = append(messages, map[string]interface{}{"role": "assistant", "content": content})
		case msg.ToolResult != nil:
			tr := msg.ToolResult
			text := ""
			if len(tr.Content) > 0 && tr.Content[0].Text != nil {
				text = tr.Content[0].Text.Text
			}
			messages = append(messages, map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"toolResult": map[string]interface{}{
							"toolUseId": tr.ToolCallID,
							"content":   []map[string]interface{}{{"text": text}},
							"status":    "success",
						},
					},
				},
			})
		}
	}
	return messages
}

func init() {
	// Register but note: requires AWS SDK integration for full functionality
	Register(&StreamProvider{
		API:          ai.APIBedrockConverseStream,
		Stream:       StreamBedrock,
		StreamSimple: StreamSimpleBedrock,
	})
}

var _ = time.Now
