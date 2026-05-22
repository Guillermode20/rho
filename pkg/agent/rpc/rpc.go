// Package rpc implements a flat JSON Lines RPC server for headless/embedded operation.
//
// Protocol:
//   - Commands are read as JSON Lines from stdin.
//   - Responses and streaming events are written as JSON Lines to stdout.
//   - Extension UI requests (select, confirm, input) block synchronously until
//     the client sends a matching extension_ui_response line.
//
// This matches the protocol defined in pi's rpc-types.ts / rpc-mode.ts.
package rpc

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/extensions"
	"github.com/earendil-works/rho/pkg/ai"
)

// ============================================================================
// Wire types — commands (stdin)
// ============================================================================

// RpcCommand is a tagged-union of all commands the client can send.
type RpcCommand struct {
	// Shared
	ID   string `json:"id,omitempty"`
	Type string `json:"type"`

	// prompt / steer / follow_up
	Message          string            `json:"message,omitempty"`
	Images           []ai.ImageContent `json:"images,omitempty"`
	StreamingBehavior string           `json:"streamingBehavior,omitempty"`

	// new_session
	ParentSession string `json:"parentSession,omitempty"`

	// set_model
	Provider string `json:"provider,omitempty"`
	ModelID  string `json:"modelId,omitempty"`

	// set_thinking_level
	Level string `json:"level,omitempty"`

	// set_steering_mode / set_follow_up_mode
	Mode string `json:"mode,omitempty"`

	// compact
	CustomInstructions string `json:"customInstructions,omitempty"`

	// set_auto_compaction / set_auto_retry
	Enabled bool `json:"enabled,omitempty"`

	// bash
	Command string `json:"command,omitempty"`

	// export_html
	OutputPath string `json:"outputPath,omitempty"`

	// switch_session
	SessionPath string `json:"sessionPath,omitempty"`

	// fork
	EntryID string `json:"entryId,omitempty"`

	// set_session_name
	Name string `json:"name,omitempty"`

	// extension_ui_response fields
	Value     string `json:"value,omitempty"`
	Confirmed bool   `json:"confirmed,omitempty"`
	Cancelled bool   `json:"cancelled,omitempty"`
}

// ============================================================================
// Wire types — responses & events (stdout)
// ============================================================================

// RpcResponse is a generic response envelope.
type RpcResponse struct {
	ID      string      `json:"id,omitempty"`
	Type    string      `json:"type"`              // "response"
	Command string      `json:"command"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// RpcExtensionUIRequest is emitted to the client when an extension needs UI.
type RpcExtensionUIRequest struct {
	Type        string   `json:"type"` // "extension_ui_request"
	ID          string   `json:"id"`
	Method      string   `json:"method"` // select | confirm | input | editor | notify | setStatus | setWidget | setTitle | set_editor_text
	Title       string   `json:"title,omitempty"`
	Message     string   `json:"message,omitempty"`
	Options     []string `json:"options,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Prefill     string   `json:"prefill,omitempty"`
	Timeout     int      `json:"timeout,omitempty"`
	// notify
	NotifyType string `json:"notifyType,omitempty"`
	// setStatus
	StatusKey  string  `json:"statusKey,omitempty"`
	StatusText *string `json:"statusText,omitempty"`
	// setWidget
	WidgetKey       string   `json:"widgetKey,omitempty"`
	WidgetLines     []string `json:"widgetLines,omitempty"`
	WidgetPlacement string   `json:"widgetPlacement,omitempty"`
	// set_editor_text
	Text string `json:"text,omitempty"`
}

// pendingUI holds a channel waiting for an extension_ui_response from the client.
type pendingUI struct {
	ch chan RpcCommand
}

// ============================================================================
// Session state (simplified — mirrors what pi's RpcSessionState exposes)
// ============================================================================

type RpcSessionState struct {
	Model                 *ai.Model        `json:"model,omitempty"`
	ThinkingLevel         ai.ThinkingLevel `json:"thinkingLevel"`
	IsStreaming           bool             `json:"isStreaming"`
	SessionID             string           `json:"sessionId"`
	SessionName           string           `json:"sessionName,omitempty"`
	MessageCount          int              `json:"messageCount"`
}

// ============================================================================
// Server
// ============================================================================

// Server is the flat JSON Lines RPC server.
type Server struct {
	// I/O — stdout is captured once at startup so logging can use os.Stderr.
	rawStdout io.Writer

	// State protected by mu.
	mu          sync.Mutex
	model       ai.Model
	apiKey      string
	thinkingLevel ai.ThinkingLevel
	systemPrompt  string
	tools       []agent.AgentTool
	sessionMgr  *agent.SessionManager
	messages    []agent.AgentMessage
	sessionID   string
	sessionName string
	isStreaming bool

	// Extension UI: map[requestID] -> channel
	pendingMu  sync.Mutex
	pending    map[string]*pendingUI
}

// NewServer creates a new RPC server and takes over stdout.
// All subsequent writes to os.Stdout are redirected to os.Stderr so they
// don't corrupt the JSON Lines framing.
func NewServer() *Server {
	// Capture the real stdout before we redirect it.
	rawOut := os.Stdout

	// Redirect go-runtime stdout writes (fmt.Print etc.) to stderr.
	// The server writes directly to rawOut via writeJSON().
	os.Stdout = os.Stderr

	return &Server{
		rawStdout: rawOut,
		pending:   make(map[string]*pendingUI),
		sessionID: newUUID(),
	}
}

// SetModel sets the AI model.
func (s *Server) SetModel(m ai.Model) {
	s.mu.Lock()
	s.model = m
	s.mu.Unlock()
}

// SetAPIKey sets the API key.
func (s *Server) SetAPIKey(key string) {
	s.mu.Lock()
	s.apiKey = key
	s.mu.Unlock()
}

// SetSystemPrompt sets the system prompt.
func (s *Server) SetSystemPrompt(prompt string) {
	s.mu.Lock()
	s.systemPrompt = prompt
	s.mu.Unlock()
}

// SetThinkingLevel sets the thinking level.
func (s *Server) SetThinkingLevel(level ai.ThinkingLevel) {
	s.mu.Lock()
	s.thinkingLevel = level
	s.mu.Unlock()
}

// SetTools sets available tools.
func (s *Server) SetTools(tools []agent.AgentTool) {
	s.mu.Lock()
	s.tools = tools
	s.mu.Unlock()
}

// SetSessionManager sets the session manager.
func (s *Server) SetSessionManager(mgr *agent.SessionManager) {
	s.mu.Lock()
	s.sessionMgr = mgr
	s.mu.Unlock()
}

// Run starts the JSON Lines read loop.
func (s *Server) Run() error {
	scanner := bufio.NewScanner(os.Stdin)
	// Increase the buffer to handle large messages with images.
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		go s.handleLine(line)
	}
	return scanner.Err()
}

// ============================================================================
// Line handler
// ============================================================================

func (s *Server) handleLine(line string) {
	var cmd RpcCommand
	if err := json.Unmarshal([]byte(line), &cmd); err != nil {
		s.writeJSON(map[string]interface{}{
			"id":      nil,
			"type":    "response",
			"command": "parse",
			"success": false,
			"error":   "Failed to parse command: " + err.Error(),
		})
		return
	}

	// Handle extension_ui_response — unblock a pending UI channel.
	if cmd.Type == "extension_ui_response" {
		s.pendingMu.Lock()
		p, ok := s.pending[cmd.ID]
		if ok {
			delete(s.pending, cmd.ID)
		}
		s.pendingMu.Unlock()
		if ok {
			p.ch <- cmd
		}
		return
	}

	resp, err := s.dispatch(cmd)
	if err != nil {
		s.writeJSON(RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: cmd.Type,
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	if resp != nil {
		s.writeJSON(resp)
	}
}

// ============================================================================
// Command dispatcher
// ============================================================================

func (s *Server) dispatch(cmd RpcCommand) (interface{}, error) {
	switch cmd.Type {

	// ── Prompting ─────────────────────────────────────────────────────────────

	case "prompt":
		// Run prompt asynchronously; emit success immediately, then stream events.
		go func() {
			s.mu.Lock()
			s.isStreaming = true
			s.mu.Unlock()

			// Acknowledge receipt.
			s.writeJSON(RpcResponse{
				ID:      cmd.ID,
				Type:    "response",
				Command: "prompt",
				Success: true,
			})

			err := s.runAgentStream(cmd.Message, cmd.Images, cmd.ID)

			s.mu.Lock()
			s.isStreaming = false
			s.mu.Unlock()

			if err != nil {
				s.writeJSON(map[string]interface{}{
					"type":  "agent_error",
					"error": err.Error(),
				})
			}
		}()
		return nil, nil

	case "steer", "follow_up":
		// Treat steer/follow_up as a new prompt in this simplified implementation.
		go func() {
			s.mu.Lock()
			s.isStreaming = true
			s.mu.Unlock()

			s.writeJSON(RpcResponse{
				ID:      cmd.ID,
				Type:    "response",
				Command: cmd.Type,
				Success: true,
			})

			s.runAgentStream(cmd.Message, cmd.Images, cmd.ID) //nolint:errcheck

			s.mu.Lock()
			s.isStreaming = false
			s.mu.Unlock()
		}()
		return nil, nil

	case "abort":
		// Best-effort — the agent loop does not have a cancel channel yet.
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: "abort",
			Success: true,
		}, nil

	case "new_session":
		s.mu.Lock()
		s.sessionID = newUUID()
		s.sessionName = ""
		s.messages = nil
		s.isStreaming = false
		s.mu.Unlock()
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: "new_session",
			Success: true,
			Data:    map[string]interface{}{"cancelled": false},
		}, nil

	// ── State ──────────────────────────────────────────────────────────────────

	case "get_state":
		s.mu.Lock()
		state := RpcSessionState{
			ThinkingLevel: s.thinkingLevel,
			IsStreaming:   s.isStreaming,
			SessionID:     s.sessionID,
			SessionName:   s.sessionName,
			MessageCount:  len(s.messages),
		}
		if s.model.Name != "" {
			m := s.model
			state.Model = &m
		}
		s.mu.Unlock()
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: "get_state",
			Success: true,
			Data:    state,
		}, nil

	// ── Model ──────────────────────────────────────────────────────────────────

	case "set_model":
		var found *ai.Model
		for _, def := range ai.DefaultModels() {
			if string(def.Provider) == cmd.Provider && def.Name == cmd.ModelID {
				m := ai.Model{
					API:      def.API,
					Provider: def.Provider,
					Name:     def.Name,
					BaseURL:  def.BaseURL,
				}
				found = &m
				break
			}
		}
		if found == nil {
			return &RpcResponse{
				ID:      cmd.ID,
				Type:    "response",
				Command: "set_model",
				Success: false,
				Error:   fmt.Sprintf("Model not found: %s/%s", cmd.Provider, cmd.ModelID),
			}, nil
		}
		s.mu.Lock()
		s.model = *found
		s.mu.Unlock()
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: "set_model",
			Success: true,
			Data:    found,
		}, nil

	case "cycle_model":
		defs := ai.DefaultModels()
		s.mu.Lock()
		cur := s.model
		s.mu.Unlock()
		if len(defs) == 0 {
			return &RpcResponse{
				ID: cmd.ID, Type: "response", Command: "cycle_model", Success: true, Data: nil,
			}, nil
		}
		nextIdx := 0
		for i, def := range defs {
			if def.Name == cur.Name && string(def.Provider) == string(cur.Provider) {
				nextIdx = (i + 1) % len(defs)
				break
			}
		}
		next := defs[nextIdx]
		nextModel := ai.Model{
			API: next.API, Provider: next.Provider, Name: next.Name, BaseURL: next.BaseURL,
		}
		s.mu.Lock()
		s.model = nextModel
		s.mu.Unlock()
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: "cycle_model",
			Success: true,
			Data: map[string]interface{}{
				"model":         nextModel,
				"thinkingLevel": s.thinkingLevel,
				"isScoped":      false,
			},
		}, nil

	case "get_available_models":
		defs := ai.DefaultModels()
		models := make([]ai.Model, 0, len(defs))
		for _, d := range defs {
			models = append(models, ai.Model{
				API: d.API, Provider: d.Provider, Name: d.Name, BaseURL: d.BaseURL,
			})
		}
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: "get_available_models",
			Success: true,
			Data:    map[string]interface{}{"models": models},
		}, nil

	// ── Thinking ───────────────────────────────────────────────────────────────

	case "set_thinking_level":
		s.mu.Lock()
		s.thinkingLevel = ai.ThinkingLevel(cmd.Level)
		s.mu.Unlock()
		return &RpcResponse{
			ID: cmd.ID, Type: "response", Command: "set_thinking_level", Success: true,
		}, nil

	case "cycle_thinking_level":
		levels := []ai.ThinkingLevel{"off", "minimal", "low", "medium", "high", "xhigh"}
		s.mu.Lock()
		cur := s.thinkingLevel
		s.mu.Unlock()
		nextIdx := 0
		for i, l := range levels {
			if l == cur {
				nextIdx = (i + 1) % len(levels)
				break
			}
		}
		next := levels[nextIdx]
		s.mu.Lock()
		s.thinkingLevel = next
		s.mu.Unlock()
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: "cycle_thinking_level",
			Success: true,
			Data:    map[string]interface{}{"level": string(next)},
		}, nil

	// ── Queue modes (stub) ─────────────────────────────────────────────────────

	case "set_steering_mode":
		return &RpcResponse{
			ID: cmd.ID, Type: "response", Command: "set_steering_mode", Success: true,
		}, nil

	case "set_follow_up_mode":
		return &RpcResponse{
			ID: cmd.ID, Type: "response", Command: "set_follow_up_mode", Success: true,
		}, nil

	// ── Compaction (stub) ──────────────────────────────────────────────────────

	case "compact":
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: "compact",
			Success: true,
			Data:    map[string]interface{}{"compacted": false, "reason": "not_needed"},
		}, nil

	case "set_auto_compaction":
		return &RpcResponse{
			ID: cmd.ID, Type: "response", Command: "set_auto_compaction", Success: true,
		}, nil

	// ── Retry (stub) ───────────────────────────────────────────────────────────

	case "set_auto_retry":
		return &RpcResponse{
			ID: cmd.ID, Type: "response", Command: "set_auto_retry", Success: true,
		}, nil

	case "abort_retry":
		return &RpcResponse{
			ID: cmd.ID, Type: "response", Command: "abort_retry", Success: true,
		}, nil

	// ── Bash ───────────────────────────────────────────────────────────────────

	case "bash":
		// Execute the bash command and return result.
		result, execErr := runBashCommand(cmd.Command)
		if execErr != nil {
			return &RpcResponse{
				ID:      cmd.ID,
				Type:    "response",
				Command: "bash",
				Success: false,
				Error:   execErr.Error(),
			}, nil
		}
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: "bash",
			Success: true,
			Data:    result,
		}, nil

	case "abort_bash":
		return &RpcResponse{
			ID: cmd.ID, Type: "response", Command: "abort_bash", Success: true,
		}, nil

	// ── Session ────────────────────────────────────────────────────────────────

	case "get_session_stats":
		s.mu.Lock()
		msgs := len(s.messages)
		s.mu.Unlock()
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: "get_session_stats",
			Success: true,
			Data: map[string]interface{}{
				"messageCount": msgs,
				"sessionId":    s.sessionID,
			},
		}, nil

	case "export_html":
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: "export_html",
			Success: false,
			Error:   "export_html not supported in this build",
		}, nil

	case "switch_session":
		s.mu.Lock()
		s.sessionID = newUUID()
		s.sessionName = ""
		s.messages = nil
		s.mu.Unlock()
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: "switch_session",
			Success: true,
			Data:    map[string]interface{}{"cancelled": false},
		}, nil

	case "fork":
		s.mu.Lock()
		s.sessionID = newUUID()
		s.mu.Unlock()
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: "fork",
			Success: true,
			Data:    map[string]interface{}{"text": "", "cancelled": false},
		}, nil

	case "clone":
		s.mu.Lock()
		s.sessionID = newUUID()
		s.mu.Unlock()
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: "clone",
			Success: true,
			Data:    map[string]interface{}{"cancelled": false},
		}, nil

	case "get_fork_messages":
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: "get_fork_messages",
			Success: true,
			Data:    map[string]interface{}{"messages": []interface{}{}},
		}, nil

	case "get_last_assistant_text":
		s.mu.Lock()
		var text *string
		for i := len(s.messages) - 1; i >= 0; i-- {
			if s.messages[i].Role == ai.RoleAssistant && s.messages[i].Content != "" {
				t := s.messages[i].Content
				text = &t
				break
			}
		}
		s.mu.Unlock()
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: "get_last_assistant_text",
			Success: true,
			Data:    map[string]interface{}{"text": text},
		}, nil

	case "set_session_name":
		name := cmd.Name
		if name == "" {
			return &RpcResponse{
				ID:      cmd.ID,
				Type:    "response",
				Command: "set_session_name",
				Success: false,
				Error:   "Session name cannot be empty",
			}, nil
		}
		s.mu.Lock()
		s.sessionName = name
		s.mu.Unlock()
		return &RpcResponse{
			ID: cmd.ID, Type: "response", Command: "set_session_name", Success: true,
		}, nil

	// ── Messages ───────────────────────────────────────────────────────────────

	case "get_messages":
		s.mu.Lock()
		msgs := make([]agent.AgentMessage, len(s.messages))
		copy(msgs, s.messages)
		s.mu.Unlock()
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: "get_messages",
			Success: true,
			Data:    map[string]interface{}{"messages": msgs},
		}, nil

	// ── Commands ───────────────────────────────────────────────────────────────

	case "get_commands":
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: "get_commands",
			Success: true,
			Data:    map[string]interface{}{"commands": []interface{}{}},
		}, nil

	default:
		return &RpcResponse{
			ID:      cmd.ID,
			Type:    "response",
			Command: cmd.Type,
			Success: false,
			Error:   fmt.Sprintf("Unknown command: %s", cmd.Type),
		}, nil
	}
}

// ============================================================================
// Agent streaming
// ============================================================================

func (s *Server) runAgentStream(content string, images []ai.ImageContent, reqID string) error {
	s.mu.Lock()
	model := s.model
	apiKey := s.apiKey
	systemPrompt := s.systemPrompt
	thinkingLevel := s.thinkingLevel
	existingMessages := make([]agent.AgentMessage, len(s.messages))
	copy(existingMessages, s.messages)
	allTools := s.tools
	s.mu.Unlock()

	// Emit agent_start event.
	s.writeJSON(map[string]interface{}{"type": "agent_start"})

	userMsg := agent.AgentMessage{
		Role:      ai.RoleUser,
		Content:   content,
		Images:    images,
		Timestamp: time.Now().UnixMilli(),
	}

	ctx := agent.AgentContext{
		SystemPrompt:  systemPrompt,
		Model:         model,
		Tools:         allTools,
		ThinkingLevel: thinkingLevel,
		Messages:      existingMessages,
	}

	loop := agent.NewAgentLoop(agent.AgentLoopConfig{
		Model:         model,
		SystemPrompt:  systemPrompt,
		APIKey:        apiKey,
		ThinkingLevel: thinkingLevel,
	})

	results, err := loop.Run([]agent.AgentMessage{userMsg}, ctx, func(event agent.AgentEvent) error {
		switch event.Type {
		case "text_delta":
			s.writeJSON(map[string]interface{}{"type": "text_delta", "delta": event.Delta})
		case "tool_execution_start":
			if event.ToolCall != nil {
				s.writeJSON(map[string]interface{}{
					"type": "tool_execution_start",
					"tool": event.ToolCall.Name,
					"id":   event.ToolCall.ID,
				})
			}
		case "tool_execution_end":
			if event.ToolCall != nil {
				s.writeJSON(map[string]interface{}{
					"type": "tool_execution_end",
					"tool": event.ToolCall.Name,
					"id":   event.ToolCall.ID,
				})
			}
		case "thinking":
			if event.Delta != "" {
				s.writeJSON(map[string]interface{}{"type": "thinking", "delta": event.Delta})
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Persist messages.
	s.mu.Lock()
	s.messages = append(s.messages, userMsg)
	for _, r := range results {
		if r.Role == ai.RoleAssistant {
			s.messages = append(s.messages, r)
		}
	}
	s.mu.Unlock()

	// Emit agent_end.
	s.writeJSON(map[string]interface{}{"type": "agent_end"})
	return nil
}

// ============================================================================
// Bash execution helper
// ============================================================================

func runBashCommand(command string) (map[string]interface{}, error) {
	if command == "" {
		return nil, fmt.Errorf("empty command")
	}
	// Use the same approach as the bash tool.
	result := map[string]interface{}{
		"stdout":   "",
		"stderr":   "",
		"exitCode": 0,
	}
	// Delegate to os/exec.
	out, err := executeBash(command)
	result["stdout"] = out
	if err != nil {
		result["stderr"] = err.Error()
		result["exitCode"] = 1
	}
	return result, nil
}

// ============================================================================
// Extension UI context (used by AgentSessionRuntime when wired to RPC mode)
// ============================================================================

// BuildExtensionUIContext returns an ExtensionUIContext that forwards all UI
// requests over the RPC JSON Lines protocol and blocks until responses arrive.
func (s *Server) BuildExtensionUIContext() extensions.ExtensionUIContext {
	return extensions.ExtensionUIContext{
		Select: func(title string, options []string) (string, error) {
			id := newUUID()
			ch := make(chan RpcCommand, 1)
			s.pendingMu.Lock()
			s.pending[id] = &pendingUI{ch: ch}
			s.pendingMu.Unlock()

			s.writeJSON(RpcExtensionUIRequest{
				Type:    "extension_ui_request",
				ID:      id,
				Method:  "select",
				Title:   title,
				Options: options,
			})

			resp := <-ch
			if resp.Cancelled {
				return "", nil
			}
			return resp.Value, nil
		},

		Confirm: func(title, message string) (bool, error) {
			id := newUUID()
			ch := make(chan RpcCommand, 1)
			s.pendingMu.Lock()
			s.pending[id] = &pendingUI{ch: ch}
			s.pendingMu.Unlock()

			s.writeJSON(RpcExtensionUIRequest{
				Type:    "extension_ui_request",
				ID:      id,
				Method:  "confirm",
				Title:   title,
				Message: message,
			})

			resp := <-ch
			if resp.Cancelled {
				return false, nil
			}
			return resp.Confirmed, nil
		},

		Input: func(title, placeholder string) (string, error) {
			id := newUUID()
			ch := make(chan RpcCommand, 1)
			s.pendingMu.Lock()
			s.pending[id] = &pendingUI{ch: ch}
			s.pendingMu.Unlock()

			s.writeJSON(RpcExtensionUIRequest{
				Type:        "extension_ui_request",
				ID:          id,
				Method:      "input",
				Title:       title,
				Placeholder: placeholder,
			})

			resp := <-ch
			if resp.Cancelled {
				return "", nil
			}
			return resp.Value, nil
		},

		Notify: func(message string, notifyType string) {
			s.writeJSON(RpcExtensionUIRequest{
				Type:       "extension_ui_request",
				ID:         newUUID(),
				Method:     "notify",
				Message:    message,
				NotifyType: notifyType,
			})
		},

		SetStatus: func(key, text string) {
			t := text
			s.writeJSON(RpcExtensionUIRequest{
				Type:       "extension_ui_request",
				ID:         newUUID(),
				Method:     "setStatus",
				StatusKey:  key,
				StatusText: &t,
			})
		},
	}
}

// ============================================================================
// Output helper
// ============================================================================

func (s *Server) writeJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(s.rawStdout, "{\"type\":\"error\",\"error\":\"marshal error: %s\"}\n", err.Error())
		return
	}
	s.rawStdout.Write(append(data, '\n')) //nolint:errcheck
}

// ============================================================================
// UUID helper (no external deps — uses crypto/rand)
// ============================================================================

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback: use timestamp-based pseudo-random.
		t := time.Now().UnixNano()
		for i := range b {
			b[i] = byte(t >> (uint(i) * 8))
		}
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
