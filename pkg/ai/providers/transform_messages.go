package providers

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/earendil-works/rho/pkg/ai"
)

// MessageTransformer handles cross-provider message format conversions.
type MessageTransformer struct {
	// Whether to normalize tool call IDs for Anthropic compatibility
	NormalizeToolCallIDs bool
	// Whether to strip unsupported image content
	StripUnsupportedImages bool
	// Whether to convert thinking blocks to text
	ConvertThinkingToText bool
	// Provider-specific options
	Provider string
}

// NewMessageTransformer creates a transformer with defaults for the given provider.
func NewMessageTransformer(provider string) *MessageTransformer {
	t := &MessageTransformer{
		Provider: provider,
	}

	switch provider {
	case "anthropic":
		t.NormalizeToolCallIDs = false // Anthropic has strict ID requirements
		t.StripUnsupportedImages = false
		t.ConvertThinkingToText = false
	case "openai":
		t.NormalizeToolCallIDs = false
		t.StripUnsupportedImages = false
		t.ConvertThinkingToText = true
	case "google", "google-vertex":
		t.NormalizeToolCallIDs = true
		t.StripUnsupportedImages = false
		t.ConvertThinkingToText = true
	default:
		t.NormalizeToolCallIDs = true
		t.StripUnsupportedImages = true
		t.ConvertThinkingToText = true
	}

	return t
}

// toolCallIDPattern matches the allowed pattern for Anthropic tool call IDs.
var toolCallIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

const maxToolCallIDLength = 64

// NormalizeToolCallID ensures a tool call ID is compatible with the target provider.
// Anthropic requires IDs matching ^[a-zA-Z0-9_-]+$ with max 64 chars.
// OpenAI generates IDs that can be 450+ chars with special characters like `|`.
func NormalizeToolCallID(id string, source string) string {
	if source == "anthropic" {
		// Anthropic IDs are already compatible
		return truncateID(id, maxToolCallIDLength)
	}

	// Check if already compatible
	if toolCallIDPattern.MatchString(id) && len(id) <= maxToolCallIDLength {
		return id
	}

	// Generate a clean, short ID
	return generateShortID()
}

// generateShortID creates a short random ID for tool calls.
func generateShortID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "tool_" + hex.EncodeToString(bytes)
}

// truncateID truncates a string to the given length, respecting boundaries.
func truncateID(id string, maxLen int) string {
	if len(id) <= maxLen {
		return id
	}
	return id[:maxLen]
}

// TransformMessages applies all enabled transformations to a message list.
func (t *MessageTransformer) TransformMessages(messages []ai.Message, model ai.Model) []ai.Message {
	result := make([]ai.Message, 0, len(messages))

	// Build a map of original tool call IDs to normalized IDs
	toolCallIDMap := make(map[string]string)

	// First pass: collect all tool call IDs that need normalization
	if t.NormalizeToolCallIDs {
		for _, msg := range messages {
			if msg.Assistant != nil {
				for _, block := range msg.Assistant.Content {
					if block.ToolCall != nil {
						origID := block.ToolCall.ID
						if _, exists := toolCallIDMap[origID]; !exists {
							toolCallIDMap[origID] = NormalizeToolCallID(origID, t.Provider)
						}
					}
				}
			}
		}
	}

	// Second pass: transform messages
	for _, msg := range messages {
		transformed := t.transformMessage(msg, model, toolCallIDMap)
		if transformed != nil {
			result = append(result, *transformed)
		}
	}

	return result
}

func (t *MessageTransformer) transformMessage(msg ai.Message, model ai.Model, idMap map[string]string) *ai.Message {
	// User messages pass through unchanged
	if msg.User != nil {
		return &msg
	}

	// Handle assistant messages
	if msg.Assistant != nil {
		return t.transformAssistant(msg.Assistant, idMap)
	}

	// Handle tool result messages - normalize toolCallId
	if msg.ToolResult != nil {
		return t.transformToolResult(msg.ToolResult, idMap)
	}

	return &msg
}

func (t *MessageTransformer) transformAssistant(amsg *ai.AssistantMessage, idMap map[string]string) *ai.Message {
	var newContent []ai.ContentBlock

	for _, block := range amsg.Content {
		switch {
		case block.Text != nil:
			newContent = append(newContent, block)

		case block.Thinking != nil:
			if t.ConvertThinkingToText {
				// Convert thinking blocks to text (for providers that don't support thinking)
				newContent = append(newContent, ai.ContentBlock{
					Text: &ai.TextContent{
						Type: "text",
						Text: "<thinking>" + block.Thinking.Thinking + "</thinking>",
					},
				})
			} else {
				newContent = append(newContent, block)
			}

		case block.ToolCall != nil:
			tc := *block.ToolCall
			if t.NormalizeToolCallIDs {
				if newID, ok := idMap[tc.ID]; ok {
					tc.ID = newID
				}
			}
			newContent = append(newContent, ai.ContentBlock{ToolCall: &tc})

		case block.Image != nil:
			if t.StripUnsupportedImages {
				newContent = append(newContent, ai.ContentBlock{
					Text: &ai.TextContent{
						Type: "text",
						Text: "(image omitted: provider does not support images)",
					},
				})
			} else {
				newContent = append(newContent, block)
			}
		}
	}

	result := *amsg
	result.Content = newContent
	return &ai.Message{Assistant: &result}
}

func (t *MessageTransformer) transformToolResult(tr *ai.ToolResultMessage, idMap map[string]string) *ai.Message {
	var newContent []ai.ContentBlock
	for _, block := range tr.Content {
		if block.Image != nil && t.StripUnsupportedImages {
			newContent = append(newContent, ai.ContentBlock{
				Text: &ai.TextContent{
					Type: "text",
					Text: "(tool image omitted: provider does not support images)",
				},
			})
		} else {
			newContent = append(newContent, block)
		}
	}

	result := *tr
	result.Content = newContent

	if t.NormalizeToolCallIDs {
		if newID, ok := idMap[tr.ToolCallID]; ok {
			result.ToolCallID = newID
		}
	}

	return &ai.Message{ToolResult: &result}
}

// SanitizeSurrogates removes lone surrogate pairs from a string.
// Some providers (like Anthropic) reject messages containing unpaired surrogates.
func SanitizeSurrogates(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	runes := []rune(s)
	for _, r := range runes {
		// Skip lone surrogates (0xD800-0xDFFF)
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

var _ = SanitizeSurrogates
var _ = NewMessageTransformer
