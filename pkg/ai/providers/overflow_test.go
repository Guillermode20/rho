package providers

import (
	"testing"

	"github.com/earendil-works/rho/pkg/ai"
)

func TestIsContextOverflow(t *testing.T) {
	tests := []struct {
		name          string
		message       *ai.AssistantMessage
		contextWindow int
		expected      bool
	}{
		{
			name:     "nil message",
			message:  nil,
			expected: false,
		},
		{
			name: "no error, stop reason stop, no overflow",
			message: &ai.AssistantMessage{
				StopReason: ai.StopReasonStop,
				Usage: ai.Usage{
					Input: 100,
				},
			},
			contextWindow: 1000,
			expected:      false,
		},
		{
			name: "anthropic token overflow error",
			message: &ai.AssistantMessage{
				StopReason:   ai.StopReasonError,
				ErrorMessage: "prompt is too long: 21346 tokens > 20000 maximum",
			},
			contextWindow: 20000,
			expected:      true,
		},
		{
			name: "openai context window exceeded error",
			message: &ai.AssistantMessage{
				StopReason:   ai.StopReasonError,
				ErrorMessage: "Your input exceeds the context window of this model",
			},
			contextWindow: 4096,
			expected:      true,
		},
		{
			name: "rate limit error matches non-overflow",
			message: &ai.AssistantMessage{
				StopReason:   ai.StopReasonError,
				ErrorMessage: "Rate limit exceeded. Too many requests.",
			},
			contextWindow: 4096,
			expected:      false,
		},
		{
			name: "silent overflow (z.ai style)",
			message: &ai.AssistantMessage{
				StopReason: ai.StopReasonStop,
				Usage: ai.Usage{
					Input:     1200,
					CacheRead: 50,
				},
			},
			contextWindow: 1000,
			expected:      true,
		},
		{
			name: "Xiaomi MiMo style overflow (length stop reason + zero output)",
			message: &ai.AssistantMessage{
				StopReason: ai.StopReasonLength,
				Usage: ai.Usage{
					Input:  995,
					Output: 0,
				},
			},
			contextWindow: 1000,
			expected:      true,
		},
		{
			name: "Xiaomi MiMo style, but output is not zero",
			message: &ai.AssistantMessage{
				StopReason: ai.StopReasonLength,
				Usage: ai.Usage{
					Input:  995,
					Output: 10,
				},
			},
			contextWindow: 1000,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := IsContextOverflow(tt.message, tt.contextWindow)
			if actual != tt.expected {
				t.Errorf("IsContextOverflow() = %v, expected %v", actual, tt.expected)
			}
		})
	}
}
