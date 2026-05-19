// Package aiutils provides shared utilities for the AI package.
package aiutils

import (
	"sync"
	"sync/atomic"
)

// EventType represents the type of a stream event.
type EventType string

const (
	EventStart         EventType = "start"
	EventTextStart     EventType = "text_start"
	EventTextDelta     EventType = "text_delta"
	EventTextEnd       EventType = "text_end"
	EventThinkingStart EventType = "thinking_start"
	EventThinkingDelta EventType = "thinking_delta"
	EventThinkingEnd   EventType = "thinking_end"
	EventToolCallStart EventType = "toolcall_start"
	EventToolCallDelta EventType = "toolcall_delta"
	EventToolCallEnd   EventType = "toolcall_end"
	EventDone          EventType = "done"
	EventError         EventType = "error"
)

// StreamEvent is a single typed event in the stream.
type StreamEvent struct {
	Type         EventType `json:"type"`
	ContentIndex int       `json:"contentIndex,omitempty"`
	Delta        string    `json:"delta,omitempty"`
	Content      string    `json:"content,omitempty"`
	ToolCallID   string    `json:"toolCallId,omitempty"`
	ToolName     string    `json:"toolName,omitempty"`
	StopReason   string    `json:"stopReason,omitempty"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
	UsageInput   int       `json:"usageInput,omitempty"`
	UsageOutput  int       `json:"usageOutput,omitempty"`
	ResponseID   string    `json:"responseId,omitempty"`
}

// EventStream is a channel-based async event stream.
// Producers push events, consumers iterate via Channel().
type EventStream struct {
	ch       chan StreamEvent
	done     chan struct{}
	closed   atomic.Bool
	mu       sync.Mutex
	onClose  []func()
}

// NewEventStream creates a buffered event stream.
func NewEventStream(bufferSize int) *EventStream {
	if bufferSize <= 0 {
		bufferSize = 64
	}
	return &EventStream{
		ch:   make(chan StreamEvent, bufferSize),
		done: make(chan struct{}),
	}
}

// Emit sends an event to the stream. Returns false if the stream is closed.
func (es *EventStream) Emit(event StreamEvent) bool {
	if es.closed.Load() {
		return false
	}
	select {
	case es.ch <- event:
		return true
	case <-es.done:
		return false
	}
}

// EmitTextDelta is a convenience for text delta events.
func (es *EventStream) EmitTextDelta(contentIndex int, delta string, partial interface{}) bool {
	_ = partial
	return es.Emit(StreamEvent{
		Type:         EventTextDelta,
		ContentIndex: contentIndex,
		Delta:        delta,
	})
}

// EmitToolCallStart is a convenience for tool call start.
func (es *EventStream) EmitToolCallStart(contentIndex int, toolCallID, toolName string) bool {
	return es.Emit(StreamEvent{
		Type:         EventToolCallStart,
		ContentIndex: contentIndex,
		ToolCallID:   toolCallID,
		ToolName:     toolName,
	})
}

// EmitToolCallDelta streams partial JSON for a tool call.
func (es *EventStream) EmitToolCallDelta(contentIndex int, delta string) bool {
	return es.Emit(StreamEvent{
		Type:         EventToolCallDelta,
		ContentIndex: contentIndex,
		Delta:        delta,
	})
}

// EmitToolCallEnd signals a complete tool call.
func (es *EventStream) EmitToolCallEnd(contentIndex int, toolCallID, toolName string) bool {
	return es.Emit(StreamEvent{
		Type:         EventToolCallEnd,
		ContentIndex: contentIndex,
		ToolCallID:   toolCallID,
		ToolName:     toolName,
	})
}

// EmitDone signals successful completion.
func (es *EventStream) EmitDone(stopReason, responseID string, usageInput, usageOutput int) bool {
	return es.Emit(StreamEvent{
		Type:        EventDone,
		StopReason:  stopReason,
		ResponseID:  responseID,
		UsageInput:  usageInput,
		UsageOutput: usageOutput,
	})
}

// EmitError signals an error.
func (es *EventStream) EmitError(errorMessage string) bool {
	return es.Emit(StreamEvent{
		Type:         EventError,
		ErrorMessage: errorMessage,
	})
}

// Channel returns a read-only channel for consumers.
func (es *EventStream) Channel() <-chan StreamEvent {
	return es.ch
}

// Done returns a channel that closes when the stream ends.
func (es *EventStream) Done() <-chan struct{} {
	return es.done
}

// Close finalizes the stream. All subsequent Emit calls return false.
func (es *EventStream) Close() {
	if es.closed.CompareAndSwap(false, true) {
		es.mu.Lock()
		for _, fn := range es.onClose {
			fn()
		}
		es.mu.Unlock()
		close(es.done)
		// Drain remaining events
		for range es.ch {
		}
	}
}

// CloseWithDone finalizes the stream and sends a final done event if possible.
func (es *EventStream) CloseWithDone(stopReason, responseID string, usageInput, usageOutput int) {
	es.EmitDone(stopReason, responseID, usageInput, usageOutput)
	es.Close()
}

// CloseWithError finalizes the stream with an error event.
func (es *EventStream) CloseWithError(errorMessage string) {
	es.EmitError(errorMessage)
	es.Close()
}

// OnClose registers a callback for when the stream closes.
func (es *EventStream) OnClose(fn func()) {
	es.mu.Lock()
	es.onClose = append(es.onClose, fn)
	es.mu.Unlock()
}

// Consume reads all events from the stream until done, calling fn for each.
func (es *EventStream) Consume(fn func(StreamEvent) error) error {
	for event := range es.ch {
		if err := fn(event); err != nil {
			return err
		}
		if event.Type == EventDone || event.Type == EventError {
			break
		}
	}
	return nil
}

// Collect reads all events into a slice.
func (es *EventStream) Collect() []StreamEvent {
	var events []StreamEvent
	for event := range es.ch {
		events = append(events, event)
		if event.Type == EventDone || event.Type == EventError {
			break
		}
	}
	return events
}

// NewCallbackStream creates a stream that invokes a callback for each event,
// then collects the final result.
func NewCallbackStream(bufferSize int) *CallbackStream {
	return &CallbackStream{
		EventStream: NewEventStream(bufferSize),
	}
}

// CallbackStream wraps EventStream with a typed result.
type CallbackStream struct {
	*EventStream
	result interface{}
}

// SetResult stores the final result.
func (cs *CallbackStream) SetResult(result interface{}) {
	cs.result = result
}

// Result returns the stored result.
func (cs *CallbackStream) Result() interface{} {
	return cs.result
}
