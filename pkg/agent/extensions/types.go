// Package extensions implements the plugin/extension system for rho.
//
// Extensions can:
//   - Subscribe to agent lifecycle events
//   - Register custom LLM-callable tools
//   - Register slash commands, keyboard shortcuts, and CLI flags
//   - Interact with the user via UI primitives (select, confirm, input)
//   - Register custom message renderers
//   - Provide custom AI providers with model definitions
//   - Load resources: skills, prompts, themes
package extensions

import (
	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/ai"
	"github.com/earendil-works/rho/pkg/tui"
)

// ============================================================================
// Tool Types
// ============================================================================

// ToolDefinition describes a custom tool for extensions.
type ToolDefinition struct {
	Name        string      `json:"name"`
	Label       string      `json:"label"`
	Description string      `json:"description"`
	PromptSnippet string    `json:"promptSnippet,omitempty"`
	PromptGuidelines []string `json:"promptGuidelines,omitempty"`
	Parameters  interface{} `json:"parameters"` // JSON schema
	Execute     func(args map[string]interface{}) (string, bool, error) `json:"-"`
}

// DefineTool creates a tool definition with proper type inference.
func DefineTool(name, description string, params interface{}, execute func(args map[string]interface{}) (string, bool, error)) ToolDefinition {
	return ToolDefinition{
		Name:        name,
		Label:       name,
		Description: description,
		Parameters:  params,
		Execute:     execute,
	}
}

// ============================================================================
// Event Types
// ============================================================================

// SessionReason describes why a session event occurred.
type SessionReason string

const (
	SessionStartup SessionReason = "startup"
	SessionReload  SessionReason = "reload"
	SessionNew     SessionReason = "new"
	SessionResume  SessionReason = "resume"
	SessionFork    SessionReason = "fork"
	SessionQuit    SessionReason = "quit"
)

// SessionStartEvent fires when a session starts.
type SessionStartEvent struct {
	Type              SessionReason
	PreviousSessionFile string
}

// SessionShutdownEvent fires when a session shuts down.
type SessionShutdownEvent struct {
	Reason      SessionReason
	TargetSessionFile string
}

// AgentStartEvent fires when the agent loop starts.
type AgentStartEvent struct {
	Type string
}

// AgentEndEvent fires when the agent loop ends.
type AgentEndEvent struct {
	Type     string
	Messages []agent.AgentMessage
}

// TurnStartEvent fires at the start of each turn.
type TurnStartEvent struct {
	TurnIndex int
}

// TurnEndEvent fires at the end of each turn.
type TurnEndEvent struct {
	TurnIndex    int
	Message      agent.AgentMessage
	ToolResults  []ai.ToolResultMessage
}

// ContextEvent fires before each LLM call.
type ContextEvent struct {
	Messages []agent.AgentMessage
}

// BeforeProviderRequestEvent fires before a provider request is sent.
type BeforeProviderRequestEvent struct {
	Payload interface{}
}

// AfterProviderResponseEvent fires after a provider response is received.
type AfterProviderResponseEvent struct {
	Status  int
	Headers map[string]string
}

// BeforeAgentStartEvent fires before the agent starts processing.
type BeforeAgentStartEvent struct {
	Prompt        string
	SystemPrompt  string
}

// InputEvent fires when user input is received.
type InputEvent struct {
	Text   string
	Source string // "interactive", "rpc", "extension"
}

// InputEventResult controls what happens with the input.
type InputEventResult struct {
	Action string // "continue", "transform", "handled"
	Text   string
}

// ToolCallEvent fires before a tool executes.
type ToolCallEvent struct {
	ToolCallID string
	ToolName   string
	Input      map[string]interface{}
}

// ToolCallEventResult controls tool execution.
type ToolCallEventResult struct {
	Block  bool
	Reason string
	Input  map[string]interface{}
}

// ToolResultEvent fires after a tool executes.
type ToolResultEvent struct {
	ToolCallID string
	ToolName   string
	Input      map[string]interface{}
	Content    string
	IsError    bool
}

// UserBashEvent fires when a user bash command is run.
type UserBashEvent struct {
	Command           string
	ExcludeFromContext bool
}

// ============================================================================
// Extension Context
// ============================================================================

// ExtensionUIContext provides UI primitives for extensions.
type ExtensionUIContext struct {
	Select    func(title string, options []string) (string, error)
	Confirm   func(title, message string) (bool, error)
	Input     func(title, placeholder string) (string, error)
	Notify    func(message string, msgType string)
	SetStatus func(key, text string)
}

// ExtensionContext is passed to all extension event handlers.
type ExtensionContext struct {
	UI             ExtensionUIContext
	HasUI          bool
	CWD            string
	Model          *ai.Model
	Abort          func()
	Shutdown       func()
	AgentLoop      *agent.AgentLoop
	ExtensionRuntime *Runtime
}

// Collection of event handler types.
type (
	SessionStartHandler         func(ctx ExtensionContext, event SessionStartEvent) error
	SessionShutdownHandler      func(ctx ExtensionContext, event SessionShutdownEvent) error
	AgentStartHandler           func(ctx ExtensionContext) error
	AgentEndHandler             func(ctx ExtensionContext, event AgentEndEvent) error
	TurnStartHandler            func(ctx ExtensionContext, event TurnStartEvent) error
	TurnEndHandler              func(ctx ExtensionContext, event TurnEndEvent) error
	ContextHandler              func(ctx ExtensionContext, event ContextEvent) ([]agent.AgentMessage, error)
	BeforeProviderRequestHandler func(ctx ExtensionContext, event BeforeProviderRequestEvent) (interface{}, error)
	BeforeAgentStartHandler     func(ctx ExtensionContext, event BeforeAgentStartEvent) error
	InputHandler                func(ctx ExtensionContext, event InputEvent) (*InputEventResult, error)
	ToolCallHandler             func(ctx ExtensionContext, event ToolCallEvent) (*ToolCallEventResult, error)
	ToolResultHandler           func(ctx ExtensionContext, event ToolResultEvent) error
	UserBashHandler             func(ctx ExtensionContext, event UserBashEvent) error
)

// ============================================================================
// Slash Commands
// ============================================================================

// SlashCommand defines a slash command registered by an extension.
type SlashCommand struct {
	Name        string
	Description string
	Args        []string
	Handler     func(ctx ExtensionContext, args []string) error
}

// RegisteredCommand is a command that has been registered with the system.
type RegisteredCommand struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Args        []string `json:"args,omitempty"`
	Handler     func(ctx ExtensionContext, args []string) error `json:"-"`
}

// ============================================================================
// Custom Provider Config
// ============================================================================

// ProviderConfig defines a custom AI provider from an extension.
type ProviderConfig struct {
	Name    string       `json:"name"`
	API     ai.API       `json:"api"`
	BaseURL string       `json:"baseUrl"`
	APIKey  string       `json:"apiKey,omitempty"`
	Models  []ai.Model   `json:"models"`
}

// ============================================================================
// Message Renderers
// ============================================================================

// MessageRenderer renders custom message types in the UI.
type MessageRenderer struct {
	Type     string
	Render   func(msg agent.AgentMessage, width int) []string
}

// ============================================================================
// Extension Flags & Shortcuts
// ============================================================================

// ExtensionFlag is a CLI flag registered by an extension.
type ExtensionFlag struct {
	Name        string
	Description string
	Default     string
	Handler     func(value string) error
}

// ExtensionShortcut is a keyboard shortcut registered by an extension.
type ExtensionShortcut struct {
	Key         tui.KeyID
	Description string
	Handler     func() error
}

// ============================================================================
// Extension Definition
// ============================================================================

// ExtensionDef is the complete extension definition that packages provide.
type ExtensionDef struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version,omitempty"`

	// Event handlers
	OnSessionStart         SessionStartHandler
	OnSessionShutdown      SessionShutdownHandler
	OnAgentStart           AgentStartHandler
	OnAgentEnd             AgentEndHandler
	OnTurnStart            TurnStartHandler
	OnTurnEnd              TurnEndHandler
	OnContext              ContextHandler
	OnBeforeProviderRequest BeforeProviderRequestHandler
	OnBeforeAgentStart     BeforeAgentStartHandler
	OnInput                InputHandler
	OnToolCall             ToolCallHandler
	OnToolResult           ToolResultHandler
	OnUserBash             UserBashHandler

	// Registered items
	CustomTools      []ToolDefinition
	CustomProviders  []ProviderConfig
	SlashCommands    []SlashCommand
	Keybindings      []ExtensionShortcut
	CLIFlags         []ExtensionFlag
	MessageRenderers []MessageRenderer
}

// ============================================================================
// Resource Loading
// ============================================================================

// ResourcesDiscoverEvent fires at startup to discover extension resources.
type ResourcesDiscoverEvent struct {
	CWD    string
	Reason string // "startup" | "reload"
}

// ResourcesDiscoverResult returns discovered resource paths.
type ResourcesDiscoverResult struct {
	SkillPaths  []string
	PromptPaths []string
	ThemePaths  []string
}

// ============================================================================
// Extension Errors
// ============================================================================

// ExtensionError wraps errors from extension handlers.
type ExtensionError struct {
	ExtensionName string
	Message       string
	Err           error
}

func (e *ExtensionError) Error() string {
	if e.Err != nil {
		return e.ExtensionName + ": " + e.Message + ": " + e.Err.Error()
	}
	return e.ExtensionName + ": " + e.Message
}

func (e *ExtensionError) Unwrap() error { return e.Err }
