package sdk

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
)

// Context provides access to host UI primitives.
type Context struct {
	sdk *SDK
}

// HasUI returns true if the host supports interactive UI operations.
func (c Context) HasUI() bool {
	return c.sdk.hasUI
}

// Confirm displays a confirmation dialog on the host.
func (c Context) Confirm(title, message string) (bool, error) {
	return c.sdk.confirm(title, message)
}

// Select displays a selection menu on the host.
func (c Context) Select(title string, options []string) (string, error) {
	return c.sdk.selectOption(title, options)
}

// Input displays a text input prompt on the host.
func (c Context) Input(title, placeholder string) (string, error) {
	return c.sdk.input(title, placeholder)
}

// Notify sends a notification message to the host.
func (c Context) Notify(message string, msgType string) {
	c.sdk.notify(message, msgType)
}

// SetStatus updates the status bar text on the host.
func (c Context) SetStatus(key, text string) {
	c.sdk.setStatus(key, text)
}

// ToolHandler defines the callback for tool execution.
type ToolHandler func(ctx Context, args map[string]interface{}) (string, bool, error)

// CommandHandler defines the callback for slash command execution.
type CommandHandler func(ctx Context, args []string) error

// SDK is the main entrypoint for Go extensions.
type SDK struct {
	id       string
	tools    map[string]toolDef
	commands map[string]CommandHandler
	hasUI    bool

	mu      sync.Mutex
	pending map[string]chan *RPCMessage
	nextID  uint64
}

type toolDef struct {
	Description string
	Parameters  interface{}
	Handler     ToolHandler
}

// RPCMessage represents a JSON-RPC 2.0 message.
type RPCMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *RPCError        `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// New creates a new SDK instance.
func New(id string) *SDK {
	return &SDK{
		id:       id,
		tools:    make(map[string]toolDef),
		commands: make(map[string]CommandHandler),
		pending:  make(map[string]chan *RPCMessage),
	}
}

// SetUI enables UI capabilities for this extension.
func (s *SDK) SetUI(enabled bool) {
	s.hasUI = enabled
}

// Tool registers a tool handler.
func (s *SDK) Tool(name, description string, params interface{}, handler ToolHandler) {
	s.tools[name] = toolDef{
		Description: description,
		Parameters:  params,
		Handler:     handler,
	}
}

// Command registers a slash command handler.
func (s *SDK) Command(name string, handler CommandHandler) {
	s.commands[name] = handler
}

// Run begins the stdio JSON-RPC message loop.
func (s *SDK) Run() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg RPCMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		if msg.Result != nil || msg.Error != nil || (msg.ID != nil && msg.Method == "") {
			var idStr string
			if msg.ID != nil {
				idStr = string(*msg.ID)
			}
			s.mu.Lock()
			ch, ok := s.pending[idStr]
			if ok {
				delete(s.pending, idStr)
				ch <- &msg
			}
			s.mu.Unlock()
		} else if msg.Method != "" {
			go s.handleRequest(msg)
		}
	}
}

func (s *SDK) handleRequest(msg RPCMessage) {
	var result interface{}
	var rpcErr *RPCError

	ctx := Context{sdk: s}

	switch msg.Method {
	case "initialize":
		var toolsList []map[string]interface{}
		for id, t := range s.tools {
			toolsList = append(toolsList, map[string]interface{}{
				"id":          id,
				"description": t.Description,
				"parameters":  t.Parameters,
			})
		}

		var commandsList []map[string]interface{}
		for name := range s.commands {
			commandsList = append(commandsList, map[string]interface{}{
				"name":        name,
				"description": "Slash command " + name,
			})
		}

		result = map[string]interface{}{
			"tools":    toolsList,
			"commands": commandsList,
		}

	case "tool.call":
		var params struct {
			Tool  string                 `json:"tool"`
			Input map[string]interface{} `json:"input"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			rpcErr = &RPCError{Code: -32602, Message: "Invalid params: " + err.Error()}
		} else {
			t, ok := s.tools[params.Tool]
			if !ok {
				rpcErr = &RPCError{Code: -32601, Message: "Tool not found: " + params.Tool}
			} else {
				content, isError, err := t.Handler(ctx, params.Input)
				if err != nil {
					rpcErr = &RPCError{Code: -32603, Message: err.Error()}
				} else {
					result = map[string]interface{}{
						"content": content,
						"isError": isError,
					}
				}
			}
		}

	case "command.call":
		var params struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			rpcErr = &RPCError{Code: -32602, Message: "Invalid params: " + err.Error()}
		} else {
			cmd, ok := s.commands[params.Command]
			if !ok {
				rpcErr = &RPCError{Code: -32601, Message: "Command not found: " + params.Command}
			} else {
				if err := cmd(ctx, params.Args); err != nil {
					rpcErr = &RPCError{Code: -32603, Message: err.Error()}
				} else {
					result = "ok"
				}
			}
		}

	case "lifecycle.event":
		result = "ok"

	default:
		rpcErr = &RPCError{Code: -32601, Message: "Method not found: " + msg.Method}
	}

	if msg.ID != nil {
		resp := RPCMessage{
			JSONRPC: "2.0",
			ID:      msg.ID,
		}
		if rpcErr != nil {
			resp.Error = rpcErr
		} else {
			resBytes, _ := json.Marshal(result)
			resp.Result = json.RawMessage(resBytes)
		}
		_ = s.send(&resp)
	}
}

func (s *SDK) callHost(method string, params interface{}) (*RPCMessage, error) {
	s.mu.Lock()
	id := s.nextID
	s.nextID++
	s.mu.Unlock()

	idBytes, _ := json.Marshal(id)
	idRaw := json.RawMessage(idBytes)

	pBytes, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	req := RPCMessage{
		JSONRPC: "2.0",
		ID:      &idRaw,
		Method:  method,
		Params:  json.RawMessage(pBytes),
	}

	ch := make(chan *RPCMessage, 1)
	idStr := string(idRaw)

	s.mu.Lock()
	s.pending[idStr] = ch
	s.mu.Unlock()

	if err := s.send(&req); err != nil {
		s.mu.Lock()
		delete(s.pending, idStr)
		s.mu.Unlock()
		return nil, err
	}

	return <-ch, nil
}

func (s *SDK) send(msg *RPCMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(data, '\n'))
	return err
}

func (s *SDK) confirm(title, message string) (bool, error) {
	resp, err := s.callHost("ui.confirm", map[string]interface{}{
		"title":   title,
		"message": message,
	})
	if err != nil {
		return false, err
	}
	var res bool
	err = json.Unmarshal(resp.Result, &res)
	return res, err
}

func (s *SDK) selectOption(title string, options []string) (string, error) {
	resp, err := s.callHost("ui.select", map[string]interface{}{
		"title":   title,
		"options": options,
	})
	if err != nil {
		return "", err
	}
	var res string
	err = json.Unmarshal(resp.Result, &res)
	return res, err
}

func (s *SDK) input(title, placeholder string) (string, error) {
	resp, err := s.callHost("ui.input", map[string]interface{}{
		"title":       title,
		"placeholder": placeholder,
	})
	if err != nil {
		return "", err
	}
	var res string
	err = json.Unmarshal(resp.Result, &res)
	return res, err
}

func (s *SDK) notify(message, msgType string) {
	msg := RPCMessage{
		JSONRPC: "2.0",
		Method:  "ui.notify",
	}
	pBytes, _ := json.Marshal(map[string]interface{}{
		"message": message,
		"type":    msgType,
	})
	msg.Params = json.RawMessage(pBytes)
	_ = s.send(&msg)
}

func (s *SDK) setStatus(key, text string) {
	msg := RPCMessage{
		JSONRPC: "2.0",
		Method:  "ui.setStatus",
	}
	pBytes, _ := json.Marshal(map[string]interface{}{
		"key":  key,
		"text": text,
	})
	msg.Params = json.RawMessage(pBytes)
	_ = s.send(&msg)
}
