package ai

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
)

// EventStreamReader reads Server-Sent Events (SSE) from a stream.
type EventStreamReader struct {
	scanner *bufio.Scanner
	body    io.ReadCloser
}

// NewEventStreamReader creates a new SSE reader.
func NewEventStreamReader(body io.ReadCloser) *EventStreamReader {
	return &EventStreamReader{
		scanner: bufio.NewScanner(body),
		body:    body,
	}
}

// Event represents a single SSE event.
type Event struct {
	Event string
	Data  string
	ID    string
}

// ReadEvent reads the next event from the stream.
func (r *EventStreamReader) ReadEvent() (*Event, error) {
	var event Event
	for r.scanner.Scan() {
		line := r.scanner.Text()
		if line == "" {
			// Empty line = end of event
			if event.Data != "" || event.Event != "" {
				return &event, nil
			}
			continue
		}

		if strings.HasPrefix(line, "event: ") {
			event.Event = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			event.Data = strings.TrimPrefix(line, "data: ")
		} else if strings.HasPrefix(line, "id: ") {
			event.ID = strings.TrimPrefix(line, "id: ")
		}
	}

	if err := r.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

// Close closes the event stream reader.
func (r *EventStreamReader) Close() error {
	return r.body.Close()
}

// doHTTPRequest performs an HTTP request with the given options.
func doHTTPRequest(ctx context.Context, method, url string, headers map[string]string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// sendStreamingRequest sends a streaming HTTP request and calls the callback for each event.
func sendStreamingRequest(
	ctx context.Context,
	method, url string,
	headers map[string]string,
	requestBody interface{},
	onEvent func(event Event) error,
) error {
	resp, err := doHTTPRequest(ctx, method, url, headers, requestBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	reader := NewEventStreamReader(resp.Body)
	defer reader.Close()

	for {
		event, err := reader.ReadEvent()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read event: %w", err)
		}
		if event != nil {
			if err := onEvent(*event); err != nil {
				return err
			}
		}
	}

	return nil
}

// StreamCompletion is a helper for simple text completion streaming.
func StreamCompletion(
	ctx context.Context,
	url string,
	headers map[string]string,
	requestBody map[string]interface{},
	onDelta func(delta string) error,
	onDone func() error,
) error {
	return sendStreamingRequest(ctx, "POST", url, headers, requestBody, func(event Event) error {
		if event.Data == "[DONE]" {
			if onDone != nil {
				return onDone()
			}
			return nil
		}

		if onDelta != nil {
			return onDelta(event.Data)
		}
		return nil
	})
}
