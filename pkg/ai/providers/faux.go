package providers

import (
	"time"

	"github.com/earendil-works/rho/pkg/ai"
)

// FauxOptions configures the faux provider's fake response.
type FauxOptions struct {
	ai.StreamOptions
	// FakeResponse is the text to return as the assistant's response.
	FakeResponse string `json:"fakeResponse,omitempty"`
	// FakeToolCalls causes the provider to return fake tool calls instead of text.
	FakeToolCalls []ai.ToolCall `json:"fakeToolCalls,omitempty"`
	// FakeStopReason sets the stop reason.
	FakeStopReason ai.StopReason `json:"fakeStopReason,omitempty"`
	// Latency simulates network delay in milliseconds.
	Latency int `json:"latency,omitempty"`
}

// StreamFaux streams a fake response for testing.
func StreamFaux(model ai.Model, ctx ai.Context, options *ai.StreamOptions, callback ai.StreamEventCallback) error {
	opts := &FauxOptions{}
	if options != nil {
		opts.StreamOptions = *options
	}
	return streamFaux(model, ctx, opts, callback)
}

// StreamSimpleFaux is the simple version.
func StreamSimpleFaux(model ai.Model, ctx ai.Context, options *ai.SimpleStreamOptions, callback ai.StreamEventCallback) error {
	opts := &FauxOptions{}
	if options != nil {
		opts.StreamOptions = options.StreamOptions
	}
	return streamFaux(model, ctx, opts, callback)
}

func streamFaux(model ai.Model, ctx ai.Context, opts *FauxOptions, callback ai.StreamEventCallback) error {
	if opts.Latency > 0 {
		time.Sleep(time.Duration(opts.Latency) * time.Millisecond)
	}

	responseText := opts.FakeResponse
	if responseText == "" {
		responseText = "This is a faux response for testing purposes."
	}
	stopReason := opts.FakeStopReason
	if stopReason == "" {
		stopReason = ai.StopReasonStop
	}

	msg := ai.NewAssistantMessage(model.API, model.Provider, model.Name)
	msg.StopReason = stopReason

	// Emit start event
	callback(ai.StreamEvent{
		Type:    "start",
		Partial: &msg,
	})

	// Emit text content
	if len(opts.FakeToolCalls) > 0 {
		msg.StopReason = ai.StopReasonToolUse
		for i, tc := range opts.FakeToolCalls {
			tc.Type = "toolCall"
			msg.Content = append(msg.Content, ai.ContentBlock{ToolCall: &tc})
			callback(ai.StreamEvent{
				Type:         "toolcall_start",
				ContentIndex: i,
			})
			callback(ai.StreamEvent{
				Type:         "toolcall_end",
				ContentIndex: i,
				ToolCall:     &tc,
			})
			_ = i
		}
	} else {
		msg.Content = append(msg.Content, ai.ContentBlock{
			Text: &ai.TextContent{Type: "text", Text: responseText},
		})
		// Emit text in chunks to simulate streaming
		chunkSize := 3
		for i := 0; i < len(responseText); i += chunkSize {
			end := i + chunkSize
			if end > len(responseText) {
				end = len(responseText)
			}
			callback(ai.StreamEvent{
				Type:  "text_delta",
				Delta: responseText[i:end],
			})
		}
	}

	// Simulate usage
	msg.Usage.Input = 10
	msg.Usage.Output = 20
	msg.Usage.TotalTokens = 30

	callback(ai.StreamEvent{
		Type:    "done",
		Message: &msg,
	})

	return nil
}

// FauxModelByName creates a faux model for testing.
func FauxModelByName(name string) ai.Model {
	return ai.Model{
		API:      "faux",
		Provider: "faux",
		Name:     name,
	}
}

func init() {
	Register(&StreamProvider{
		API:          "faux",
		Stream:       StreamFaux,
		StreamSimple: StreamSimpleFaux,
	})
}
