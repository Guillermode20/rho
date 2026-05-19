// Package rpc implements a JSON-RPC server for external tool integration.
package rpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/ai"
	"github.com/earendil-works/rho/pkg/ai/providers"
)

const (
	ErrParse      = -32700
	ErrInvalidReq = -32600
	ErrMethod     = -32601
	ErrParams     = -32602
	ErrInternal   = -32603
)

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      *int        `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *ErrorObj   `json:"error,omitempty"`
}

// ErrorObj is a JSON-RPC error object.
type ErrorObj struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *ErrorObj) Error() string { return e.Message }

// Server is the JSON-RPC server.
type Server struct {
	reader     *bufio.Scanner
	writer     io.Writer
	mu         sync.Mutex
	handlers   map[string]func(Request) error
	model      ai.Model
	apiKey     string
	tools      []agent.AgentTool
	sessionMgr *agent.SessionManager
}

// NewServer creates a new RPC server.
func NewServer() *Server {
	s := &Server{
		reader:   bufio.NewScanner(os.Stdin),
		writer:   os.Stdout,
		handlers: make(map[string]func(Request) error),
	}
	s.registerMethods()
	return s
}

// SetModel sets the AI model.
func (s *Server) SetModel(m ai.Model) { s.model = m }

// SetAPIKey sets the API key.
func (s *Server) SetAPIKey(key string) { s.apiKey = key }

// SetTools sets available tools.
func (s *Server) SetTools(tools []agent.AgentTool) { s.tools = tools }

// SetSessionManager sets the session manager.
func (s *Server) SetSessionManager(mgr *agent.SessionManager) { s.sessionMgr = mgr }

func (s *Server) registerMethods() {
	s.handlers["ping"] = func(req Request) error {
		return s.writeResult(req.ID, "pong")
	}

	s.handlers["models.list"] = func(req Request) error {
		defs := ai.DefaultModels()
		return s.writeResult(req.ID, defs)
	}

	s.handlers["tools.list"] = func(req Request) error {
		return s.writeResult(req.ID, s.tools)
	}

	s.handlers["session.list"] = func(req Request) error {
		if s.sessionMgr == nil {
			return s.writeError(req.ID, ErrInternal, "no session manager")
		}
		sessions, err := s.sessionMgr.List()
		if err != nil {
			return s.writeError(req.ID, ErrInternal, err.Error())
		}
		return s.writeResult(req.ID, sessions)
	}

	s.handlers["session.load"] = func(req Request) error {
		var params struct {
			ID string `json:"id"`
		}
		json.Unmarshal(req.Params, &params)
		if s.sessionMgr == nil {
			return s.writeError(req.ID, ErrInternal, "no session manager")
		}
		h, msgs, err := s.sessionMgr.Load(params.ID)
		if err != nil {
			return s.writeError(req.ID, ErrInternal, err.Error())
		}
		return s.writeResult(req.ID, map[string]interface{}{"header": h, "messages": msgs})
	}

	s.handlers["session.delete"] = func(req Request) error {
		var params struct{ ID string `json:"id"` }
		json.Unmarshal(req.Params, &params)
		if s.sessionMgr != nil {
			s.sessionMgr.Delete(params.ID)
		}
		return s.writeResult(req.ID, map[string]bool{"deleted": true})
	}

	s.handlers["chat"] = func(req Request) error {
		var params struct {
			Message string `json:"message"`
			System  string `json:"system,omitempty"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.writeError(req.ID, ErrParams, fmt.Sprintf("invalid params: %v", err))
		}

		ctx := ai.Context{
			SystemPrompt: params.System,
			Messages: []ai.Message{
				ai.NewUserMessage(params.Message),
			},
		}

		var responseText strings.Builder
		key := s.apiKey
		if key == "" {
			key = providers.GetEnvAPIKey("OPENAI_API_KEY", "ANTHROPIC_API_KEY")
		}

		err := providers.Stream(s.model, ctx, &ai.StreamOptions{
			APIKey: key,
		}, func(event ai.StreamEvent) error {
			if event.Type == "text_delta" {
				responseText.WriteString(event.Delta)
			}
			if event.Type == "done" && event.Message != nil {
			}
			return nil
		})

		if err != nil {
			return s.writeError(req.ID, ErrInternal, fmt.Sprintf("stream error: %v", err))
		}

		return s.writeResult(req.ID, map[string]interface{}{
			"response": responseText.String(),
		})
	}

	s.handlers["tool.execute"] = func(req Request) error {
		var params struct {
			Name string                 `json:"name"`
			Args map[string]interface{} `json:"args"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.writeError(req.ID, ErrParams, fmt.Sprintf("invalid params: %v", err))
		}
		for _, t := range s.tools {
			if t.Name == params.Name {
				content, isErr, err := t.Execute(params.Args)
				if err != nil {
					return s.writeError(req.ID, ErrInternal, err.Error())
				}
				return s.writeResult(req.ID, map[string]interface{}{
					"content": content, "isError": isErr,
				})
			}
		}
		return s.writeError(req.ID, ErrMethod, fmt.Sprintf("unknown tool: %s", params.Name))
	}

	s.handlers["agent.run"] = func(req Request) error {
		var params struct {
			Prompt string `json:"prompt"`
		}
		json.Unmarshal(req.Params, &params)
		// Simple agent run
		ctx := ai.Context{
			Messages: []ai.Message{ai.NewUserMessage(params.Prompt)},
		}
		var responseText strings.Builder
		providers.Stream(s.model, ctx, &ai.StreamOptions{APIKey: s.apiKey}, func(event ai.StreamEvent) error {
			if event.Type == "text_delta" {
				responseText.WriteString(event.Delta)
			}
			return nil
		})
		return s.writeResult(req.ID, map[string]interface{}{
			"response": responseText.String(),
		})
	}

	s.handlers["tools.list"] = func(req Request) error {
		return s.writeResult(req.ID, s.tools)
	}
}

// Run starts the RPC server loop.
func (s *Server) Run() error {
	for s.reader.Scan() {
		line := strings.TrimSpace(s.reader.Text())
		if line == "" {
			continue
		}
		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.writeError(nil, ErrParse, "parse error: "+err.Error())
			continue
		}
		if req.ID == nil {
			continue // notification, no response
		}
		s.mu.Lock()
		handler, ok := s.handlers[req.Method]
		s.mu.Unlock()
		if !ok {
			s.writeError(req.ID, ErrMethod, fmt.Sprintf("unknown method: %s", req.Method))
			continue
		}
		handler(req)
	}
	return s.reader.Err()
}

func (s *Server) writeResult(id *int, result interface{}) error {
	resp := Response{JSONRPC: "2.0", ID: id, Result: result}
	data, _ := json.Marshal(resp)
	_, err := fmt.Fprintln(s.writer, string(data))
	return err
}

func (s *Server) writeError(id *int, code int, message string) error {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &ErrorObj{Code: code, Message: message},
	}
	data, _ := json.Marshal(resp)
	_, err := fmt.Fprintln(s.writer, string(data))
	return err
}

var _ = time.Now

// StreamModel streams a response from the configured model using the registry.
func StreamModel(model ai.Model, ctx ai.Context, opts *ai.SimpleStreamOptions, callback ai.StreamEventCallback) error {
	return providers.StreamSimple(model, ctx, opts, callback)
}
