package extensions

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RPCMessage represents a JSON-RPC 2.0 message
type RPCMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *RPCError        `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// ExtensionProcess manages a single extension process and handles bidirectional JSON-RPC 2.0.
type ExtensionProcess struct {
	dir       string
	manifest  *Manifest
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	mu        sync.Mutex
	pending   map[string]chan *RPCMessage
	nextID    uint64
	isRunning bool
	ui        ExtensionUIContext
	logger    func(string)
}

// NewExtensionProcess creates a new ExtensionProcess.
func NewExtensionProcess(dir string, manifest *Manifest) *ExtensionProcess {
	return &ExtensionProcess{
		dir:      dir,
		manifest: manifest,
		pending:  make(map[string]chan *RPCMessage),
	}
}

// SetLogger sets the stderr logger.
func (ep *ExtensionProcess) SetLogger(l func(string)) {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	ep.logger = l
}

// Start spawns the process and handles the initial handshake.
func (ep *ExtensionProcess) Start(ui ExtensionUIContext) error {
	ep.mu.Lock()
	if ep.isRunning {
		ep.mu.Unlock()
		return nil
	}
	ep.mu.Unlock()

	parts := strings.Fields(ep.manifest.Entry.Command)
	if len(parts) == 0 {
		return fmt.Errorf("no entry command specified")
	}

	cmdPath := parts[0]
	if !filepath.IsAbs(cmdPath) {
		cmdPath = filepath.Join(ep.dir, cmdPath)
	}

	args := parts[1:]
	cmd := exec.Command(cmdPath, args...)
	cmd.Dir = ep.dir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	ep.mu.Lock()
	ep.cmd = cmd
	ep.stdin = stdin
	ep.stdout = stdout
	ep.stderr = stderr
	ep.pending = make(map[string]chan *RPCMessage)
	ep.isRunning = true
	ep.ui = ui
	ep.mu.Unlock()

	go ep.readStdoutLoop()
	go ep.readStderrLoop()

	// Handshake
	err = ep.initializeHandshake()
	if err != nil {
		ep.Stop()
		return fmt.Errorf("handshake failed: %w", err)
	}

	return nil
}

// Stop terminates the process.
func (ep *ExtensionProcess) Stop() {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	ep.stopUnderlying()
}

func (ep *ExtensionProcess) stopUnderlying() {
	if ep.cmd != nil && ep.cmd.Process != nil {
		_ = ep.cmd.Process.Kill()
		_ = ep.cmd.Wait()
	}
	ep.isRunning = false
	ep.cmd = nil
	ep.stdin = nil
	ep.stdout = nil
	ep.stderr = nil
}

// IsRunning checks if the process is active.
func (ep *ExtensionProcess) IsRunning() bool {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	return ep.isRunning
}

// Call sends a request and blocks waiting for the response.
func (ep *ExtensionProcess) Call(method string, params interface{}) (*RPCMessage, error) {
	ep.mu.Lock()
	if !ep.isRunning {
		ep.mu.Unlock()
		return nil, fmt.Errorf("process not running")
	}
	id := ep.nextID
	ep.nextID++
	ep.mu.Unlock()

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

	ep.mu.Lock()
	ep.pending[idStr] = ch
	ep.mu.Unlock()

	if err := ep.send(&req); err != nil {
		ep.mu.Lock()
		delete(ep.pending, idStr)
		ep.mu.Unlock()
		return nil, err
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("RPC error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
		}
		return resp, nil
	case <-time.After(30 * time.Second):
		ep.mu.Lock()
		delete(ep.pending, idStr)
		ep.mu.Unlock()
		return nil, fmt.Errorf("RPC timeout on method %s", method)
	}
}

func (ep *ExtensionProcess) send(msg *RPCMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	ep.mu.Lock()
	defer ep.mu.Unlock()
	if ep.stdin == nil {
		return fmt.Errorf("process stdin is nil")
	}
	_, err = ep.stdin.Write(append(data, '\n'))
	return err
}

func (ep *ExtensionProcess) initializeHandshake() error {
	var params struct {
		RhoVersion string `json:"rhoVersion"`
	}
	params.RhoVersion = "0.1.0"
	_, err := ep.Call("initialize", params)
	return err
}

func (ep *ExtensionProcess) readStdoutLoop() {
	scanner := bufio.NewScanner(ep.stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg RPCMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		// Response message
		if msg.Result != nil || msg.Error != nil || (msg.ID != nil && msg.Method == "") {
			var idStr string
			if msg.ID != nil {
				idStr = string(*msg.ID)
			}
			ep.mu.Lock()
			ch, ok := ep.pending[idStr]
			if ok {
				delete(ep.pending, idStr)
				ch <- &msg
			}
			ep.mu.Unlock()
		} else if msg.Method != "" {
			// Request or notification from extension
			go ep.handleIncomingRequest(msg)
		}
	}

	ep.mu.Lock()
	ep.isRunning = false
	ep.mu.Unlock()
}

func (ep *ExtensionProcess) readStderrLoop() {
	scanner := bufio.NewScanner(ep.stderr)
	for scanner.Scan() {
		line := scanner.Text()
		ep.mu.Lock()
		logger := ep.logger
		ep.mu.Unlock()
		if logger != nil {
			logger(line)
		}
	}
}

func (ep *ExtensionProcess) handleIncomingRequest(msg RPCMessage) {
	var result interface{}
	var rpcErr *RPCError

	ep.mu.Lock()
	ui := ep.ui
	ep.mu.Unlock()

	switch msg.Method {
	case "ui.select":
		var params struct {
			Title   string   `json:"title"`
			Options []string `json:"options"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			rpcErr = &RPCError{Code: -32602, Message: "Invalid params: " + err.Error()}
		} else if ui.Select == nil {
			rpcErr = &RPCError{Code: -32601, Message: "ui.select not supported"}
		} else {
			sel, err := ui.Select(params.Title, params.Options)
			if err != nil {
				rpcErr = &RPCError{Code: -32603, Message: err.Error()}
			} else {
				result = sel
			}
		}

	case "ui.confirm":
		var params struct {
			Title   string `json:"title"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			rpcErr = &RPCError{Code: -32602, Message: "Invalid params: " + err.Error()}
		} else if ui.Confirm == nil {
			rpcErr = &RPCError{Code: -32601, Message: "ui.confirm not supported"}
		} else {
			conf, err := ui.Confirm(params.Title, params.Message)
			if err != nil {
				rpcErr = &RPCError{Code: -32603, Message: err.Error()}
			} else {
				result = conf
			}
		}

	case "ui.input":
		var params struct {
			Title       string `json:"title"`
			Placeholder string `json:"placeholder"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			rpcErr = &RPCError{Code: -32602, Message: "Invalid params: " + err.Error()}
		} else if ui.Input == nil {
			rpcErr = &RPCError{Code: -32601, Message: "ui.input not supported"}
		} else {
			inp, err := ui.Input(params.Title, params.Placeholder)
			if err != nil {
				rpcErr = &RPCError{Code: -32603, Message: err.Error()}
			} else {
				result = inp
			}
		}

	case "ui.notify":
		var params struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		}
		if err := json.Unmarshal(msg.Params, &params); err == nil && ui.Notify != nil {
			ui.Notify(params.Message, params.Type)
		}
		return

	case "ui.setStatus":
		var params struct {
			Key  string `json:"key"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(msg.Params, &params); err == nil && ui.SetStatus != nil {
			ui.SetStatus(params.Key, params.Text)
		}
		return

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
		_ = ep.send(&resp)
	}
}
