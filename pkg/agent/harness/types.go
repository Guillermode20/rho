// Package harness provides a configurable agent harness that wraps the agent loop
// with session management, skills integration, file system abstraction, and event hooks.
package harness

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/earendil-works/rho/pkg/agent"
)

// ============================================================================
// Result Types
// ============================================================================

// Result is a type-safe result of a fallible operation.
type Result[TValue any, TError any] struct {
	Ok    bool
	Value TValue
	Error TError
}

// Ok creates a successful Result.
func Ok[TValue, TError any](value TValue) Result[TValue, TError] {
	return Result[TValue, TError]{Ok: true, Value: value}
}

// Err creates a failed Result.
func Err[TValue, TError any](err TError) Result[TValue, TError] {
	return Result[TValue, TError]{Error: err}
}

// Must returns the value or panics with the error. Use only in tests.
func Must[TValue any, TError any](r Result[TValue, TError]) TValue {
	if !r.Ok {
		panic(fmt.Sprintf("Result was error: %v", r.Error))
	}
	return r.Value
}

// ============================================================================
// Errors
// ============================================================================

type FileErrorCode string

const (
	FileErrAborted      FileErrorCode = "aborted"
	FileErrNotFound     FileErrorCode = "not_found"
	FileErrPermission   FileErrorCode = "permission_denied"
	FileErrNotDir       FileErrorCode = "not_directory"
	FileErrIsDir        FileErrorCode = "is_directory"
	FileErrInvalid      FileErrorCode = "invalid"
	FileErrNotSupported FileErrorCode = "not_supported"
	FileErrUnknown      FileErrorCode = "unknown"
)

// FileError represents a filesystem error.
type FileError struct {
	Code    FileErrorCode `json:"code"`
	Message string        `json:"message"`
	Path    string        `json:"path,omitempty"`
	Err     error         `json:"-"`
}

func (e *FileError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s (path: %s)", e.Code, e.Message, e.Path)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *FileError) Unwrap() error { return e.Err }

// NewFileError creates a new FileError.
func NewFileError(code FileErrorCode, message string, path string, cause error) *FileError {
	return &FileError{Code: code, Message: message, Path: path, Err: cause}
}

type ExecutionErrorCode string

const (
	ExecErrAborted      ExecutionErrorCode = "aborted"
	ExecErrTimeout      ExecutionErrorCode = "timeout"
	ExecErrShellUnavail ExecutionErrorCode = "shell_unavailable"
	ExecErrSpawn        ExecutionErrorCode = "spawn_error"
	ExecErrCallback     ExecutionErrorCode = "callback_error"
	ExecErrUnknown      ExecutionErrorCode = "unknown"
)

// ExecutionError represents a command execution error.
type ExecutionError struct {
	Code    ExecutionErrorCode `json:"code"`
	Message string             `json:"message"`
	Err     error              `json:"-"`
}

func (e *ExecutionError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
func (e *ExecutionError) Unwrap() error { return e.Err }

type CompactionErrorCode string

const (
	CompErrAborted        CompactionErrorCode = "aborted"
	CompErrSummarization  CompactionErrorCode = "summarization_failed"
	CompErrInvalidSession CompactionErrorCode = "invalid_session"
	CompErrUnknown        CompactionErrorCode = "unknown"
)

// CompactionError represents a compaction error.
type CompactionError struct {
	Code    CompactionErrorCode `json:"code"`
	Message string              `json:"message"`
	Err     error               `json:"-"`
}

func (e *CompactionError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
func (e *CompactionError) Unwrap() error { return e.Err }

type BranchSummaryErrorCode string

const (
	BranchErrAborted        BranchSummaryErrorCode = "aborted"
	BranchErrSummarization  BranchSummaryErrorCode = "summarization_failed"
	BranchErrInvalidSession BranchSummaryErrorCode = "invalid_session"
)

// BranchSummaryError represents a branch summary error.
type BranchSummaryError struct {
	Code    BranchSummaryErrorCode `json:"code"`
	Message string                 `json:"message"`
	Err     error                  `json:"-"`
}

func (e *BranchSummaryError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
func (e *BranchSummaryError) Unwrap() error { return e.Err }

type SessionErrorCode string

const (
	SessionErrNotFound     SessionErrorCode = "not_found"
	SessionErrInvalid      SessionErrorCode = "invalid_session"
	SessionErrInvalidEntry SessionErrorCode = "invalid_entry"
	SessionErrInvalidFork  SessionErrorCode = "invalid_fork_target"
	SessionErrStorage      SessionErrorCode = "storage"
	SessionErrUnknown      SessionErrorCode = "unknown"
)

// SessionError represents a session error.
type SessionError struct {
	Code    SessionErrorCode `json:"code"`
	Message string           `json:"message"`
	Err     error            `json:"-"`
}

func (e *SessionError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
func (e *SessionError) Unwrap() error { return e.Err }

type AgentHarnessErrorCode string

const (
	HarnessErrBusy          AgentHarnessErrorCode = "busy"
	HarnessErrInvalidState  AgentHarnessErrorCode = "invalid_state"
	HarnessErrInvalidArg    AgentHarnessErrorCode = "invalid_argument"
	HarnessErrSession       AgentHarnessErrorCode = "session"
	HarnessErrHook          AgentHarnessErrorCode = "hook"
	HarnessErrAuth          AgentHarnessErrorCode = "auth"
	HarnessErrCompaction    AgentHarnessErrorCode = "compaction"
	HarnessErrBranchSummary AgentHarnessErrorCode = "branch_summary"
	HarnessErrUnknown       AgentHarnessErrorCode = "unknown"
)

// AgentHarnessError represents a harness-level error.
type AgentHarnessError struct {
	Code    AgentHarnessErrorCode `json:"code"`
	Message string                `json:"message"`
	Err     error                 `json:"-"`
}

func (e *AgentHarnessError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
func (e *AgentHarnessError) Unwrap() error { return e.Err }

// ============================================================================
// File System Types
// ============================================================================

type FileKind string

const (
	FileKindFile      FileKind = "file"
	FileKindDirectory FileKind = "directory"
	FileKindSymlink   FileKind = "symlink"
)

// FileInfo represents metadata for a filesystem object.
type FileInfo struct {
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	Kind    FileKind `json:"kind"`
	Size    int64    `json:"size"`
	ModTime int64    `json:"mtimeMs"` // milliseconds since Unix epoch
}

// FileSystem defines the filesystem abstraction used by the harness.
type FileSystem interface {
	CWD() string
	AbsolutePath(path string) Result[string, *FileError]
	JoinPath(parts []string) Result[string, *FileError]
	ReadTextFile(path string) Result[string, *FileError]
	ReadTextLines(path string, maxLines int) Result[[]string, *FileError]
	ReadBinaryFile(path string) Result[[]byte, *FileError]
	WriteFile(path string, content []byte) Result[struct{}, *FileError]
	AppendFile(path string, content []byte) Result[struct{}, *FileError]
	FileInfo(path string) Result[*FileInfo, *FileError]
	ListDir(path string) Result[[]*FileInfo, *FileError]
	CanonicalPath(path string) Result[string, *FileError]
	Exists(path string) Result[bool, *FileError]
	CreateDir(path string, recursive bool) Result[struct{}, *FileError]
	Remove(path string, recursive, force bool) Result[struct{}, *FileError]
	CreateTempDir(prefix string) Result[string, *FileError]
	CreateTempFile(prefix, suffix string) Result[string, *FileError]
	Cleanup()
}

// ExecutionEnvExecOptions configures command execution.
type ExecutionEnvExecOptions struct {
	CWD         string
	Env         map[string]string
	Timeout     int // seconds
	AbortSignal chan struct{}
	OnStdout    func(chunk string)
	OnStderr    func(chunk string)
}

// Shell defines command execution capability.
type Shell interface {
	Exec(command string, options *ExecutionEnvExecOptions) Result[ExecResult, *ExecutionError]
	Cleanup()
}

// ExecResult contains the result of a command execution.
type ExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

// ExecutionEnv combines filesystem and shell capabilities.
type ExecutionEnv interface {
	FileSystem
	Shell
}

// ============================================================================
// Skill Types
// ============================================================================

// Skill defines a loaded skill from a SKILL.md file.
type Skill struct {
	Name                   string `json:"name"`
	Description            string `json:"description"`
	Content                string `json:"content"`
	FilePath               string `json:"filePath"`
	DisableModelInvocation bool   `json:"disableModelInvocation,omitempty"`
}

// SkillFrontmatter is the YAML frontmatter of a skill file.
type SkillFrontmatter struct {
	Name                   string `yaml:"name"`
	Description            string `yaml:"description"`
	DisableModelInvocation bool   `yaml:"disable-model-invocation"`
	Glob                   string `yaml:"glob"`
}

// SkillDiagnostic represents a warning from skill loading.
type SkillDiagnostic struct {
	Type    string `json:"type"` // "warning"
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path"`
}

// ============================================================================
// Prompt Template Types
// ============================================================================

// PromptTemplate defines a reusable prompt template.
type PromptTemplate struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content"`
}

// AgentHarnessResources collects resources available to the harness.
type AgentHarnessResources struct {
	PromptTemplates []PromptTemplate `json:"promptTemplates,omitempty"`
	Skills          []Skill          `json:"skills,omitempty"`
}

// ============================================================================
// Stream Options
// ============================================================================

// AgentHarnessStreamOptions configures provider requests from the harness.
type AgentHarnessStreamOptions struct {
	Transport       string                 `json:"transport,omitempty"`
	TimeoutMs       int                    `json:"timeoutMs,omitempty"`
	MaxRetries      int                    `json:"maxRetries,omitempty"`
	MaxRetryDelayMs int                    `json:"maxRetryDelayMs,omitempty"`
	Headers         map[string]string      `json:"headers,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	CacheRetention  string                 `json:"cacheRetention,omitempty"`
}

// AgentHarnessStreamOptionsPatch patches stream options per request.
type AgentHarnessStreamOptionsPatch struct {
	Transport       *string                `json:"transport,omitempty"`
	TimeoutMs       *int                   `json:"timeoutMs,omitempty"`
	MaxRetries      *int                   `json:"maxRetries,omitempty"`
	MaxRetryDelayMs *int                   `json:"maxRetryDelayMs,omitempty"`
	Headers         map[string]*string     `json:"headers,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	CacheRetention  *string                `json:"cacheRetention,omitempty"`
}

// ============================================================================
// Session Tree Types
// ============================================================================

// SessionTreeEntryBase is the base for all session tree entries.
type SessionTreeEntryBase struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	ParentID  string `json:"parentId,omitempty"`
	Timestamp string `json:"timestamp"`
}

// MessageEntry is a session tree entry for a message.
type MessageEntry struct {
	SessionTreeEntryBase
	Message agent.AgentMessage `json:"message"`
}

// ThinkingLevelChangeEntry records a thinking level change.
type ThinkingLevelChangeEntry struct {
	SessionTreeEntryBase
	ThinkingLevel string `json:"thinkingLevel"`
}

// ModelChangeEntry records a model change.
type ModelChangeEntry struct {
	SessionTreeEntryBase
	Provider string `json:"provider"`
	ModelID  string `json:"modelId"`
}

// CompactionEntry records a compaction event.
type CompactionEntry struct {
	SessionTreeEntryBase
	Summary          string      `json:"summary"`
	FirstKeptEntryID string      `json:"firstKeptEntryId"`
	TokensBefore     int         `json:"tokensBefore"`
	Details          interface{} `json:"details,omitempty"`
	FromHook         bool        `json:"fromHook,omitempty"`
}

// BranchSummaryEntry records a branch summary.
type BranchSummaryEntry struct {
	SessionTreeEntryBase
	FromID   string      `json:"fromId"`
	Summary  string      `json:"summary"`
	Details  interface{} `json:"details,omitempty"`
	FromHook bool        `json:"fromHook,omitempty"`
}

// CustomEntry stores custom extension data.
type CustomEntry struct {
	SessionTreeEntryBase
	CustomType string      `json:"customType"`
	Data       interface{} `json:"data,omitempty"`
}

// CustomMessageEntry stores a custom message type.
type CustomMessageEntry struct {
	SessionTreeEntryBase
	CustomType string      `json:"customType"`
	Content    interface{} `json:"content"`
	Details    interface{} `json:"details,omitempty"`
	Display    bool        `json:"display"`
}

// LabelEntry attaches a label to a target entry.
type LabelEntry struct {
	SessionTreeEntryBase
	TargetID string `json:"targetId"`
	Label    string `json:"label"`
}

// LeafEntry marks the current leaf in the session tree.
type LeafEntry struct {
	SessionTreeEntryBase
	TargetID string `json:"targetId"`
}

// SessionTreeEntry is a union of all entry types.
type SessionTreeEntry interface{}

// SessionContext holds the current session state.
type SessionContext struct {
	Messages      []agent.AgentMessage `json:"messages"`
	ThinkingLevel string               `json:"thinkingLevel"`
	Model         *ModelRef            `json:"model"`
}

// ModelRef references a model.
type ModelRef struct {
	Provider string `json:"provider"`
	ModelID  string `json:"modelId"`
}

// ============================================================================
// Session Storage
// ============================================================================

// SessionMetadata identifies a session.
type SessionMetadata struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
}

// JsonlSessionMetadata extends SessionMetadata for JSONL sessions.
type JsonlSessionMetadata struct {
	SessionMetadata
	CWD               string `json:"cwd"`
	Path              string `json:"path"`
	ParentSessionPath string `json:"parentSessionPath,omitempty"`
}

// SessionStorage defines the interface for session persistence.
type SessionStorage interface {
	GetMetadata() (*JsonlSessionMetadata, error)
	GetLeafID() (*string, error)
	SetLeafID(leafID *string) error
	CreateEntryID() (string, error)
	AppendEntry(entry SessionTreeEntry) error
	GetEntry(id string) (SessionTreeEntry, error)
	GetEntries() ([]SessionTreeEntry, error)
}

// ============================================================================
// Compaction Types
// ============================================================================

// CompactionSettings configures session compaction.
type CompactionSettings struct {
	Enabled          bool `json:"enabled"`
	ReserveTokens    int  `json:"reserveTokens"`
	KeepRecentTokens int  `json:"keepRecentTokens"`
}

// CompactionPreparation contains data for a compaction operation.
type CompactionPreparation struct {
	FirstKeptEntryID    string               `json:"firstKeptEntryId"`
	MessagesToSummarize []agent.AgentMessage `json:"messagesToSummarize"`
	TurnPrefixMessages  []agent.AgentMessage `json:"turnPrefixMessages"`
	IsSplitTurn         bool                 `json:"isSplitTurn"`
	TokensBefore        int                  `json:"tokensBefore"`
	PreviousSummary     string               `json:"previousSummary,omitempty"`
	Settings            CompactionSettings   `json:"settings"`
}

// CompactResult contains the result of a compaction.
type CompactResult struct {
	Summary          string      `json:"summary"`
	FirstKeptEntryID string      `json:"firstKeptEntryId"`
	TokensBefore     int         `json:"tokensBefore"`
	Details          interface{} `json:"details,omitempty"`
}

// ============================================================================
// Harness Event Types
// ============================================================================

// AgentHarnessPhase describes the current harness state.
type AgentHarnessPhase string

const (
	PhaseIdle          AgentHarnessPhase = "idle"
	PhaseTurn          AgentHarnessPhase = "turn"
	PhaseCompaction    AgentHarnessPhase = "compaction"
	PhaseBranchSummary AgentHarnessPhase = "branch_summary"
	PhaseRetry         AgentHarnessPhase = "retry"
)

// Harness-specific event types (not in agent.AgentEvent).
type (
	QueueUpdateEvent struct {
		Type     string               `json:"type"`
		Steer    []agent.AgentMessage `json:"steer"`
		FollowUp []agent.AgentMessage `json:"followUp"`
		NextTurn []agent.AgentMessage `json:"nextTurn"`
	}
	SavePointEvent struct {
		Type                string `json:"type"`
		HadPendingMutations bool   `json:"hadPendingMutations"`
	}
	AbortEvent struct {
		Type            string               `json:"type"`
		ClearedSteer    []agent.AgentMessage `json:"clearedSteer"`
		ClearedFollowUp []agent.AgentMessage `json:"clearedFollowUp"`
	}
	SettledEvent struct {
		Type          string `json:"type"`
		NextTurnCount int    `json:"nextTurnCount"`
	}
)

// Harness event type constants.
const (
	EventQueueUpdate           = "queue_update"
	EventSavePoint             = "save_point"
	EventAbort                 = "abort"
	EventSettled               = "settled"
	EventBeforeAgentStart      = "before_agent_start"
	EventContext               = "context"
	EventBeforeProviderReq     = "before_provider_request"
	EventBeforeProviderPayload = "before_provider_payload"
	EventAfterProviderResp     = "after_provider_response"
	EventToolCall              = "tool_call"
	EventToolResult            = "tool_result"
	EventSessionBeforeCompact  = "session_before_compact"
	EventSessionCompact        = "session_compact"
	EventSessionBeforeTree     = "session_before_tree"
	EventSessionTree           = "session_tree"
	EventModelSelect           = "model_select"
	EventThinkingLevelSelect   = "thinking_level_select"
	EventResourcesUpdate       = "resources_update"
)

// ============================================================================
// Tree Preparation / Navigation
// ============================================================================

// TreePreparation contains data for tree navigation.
type TreePreparation struct {
	TargetID            string  `json:"targetId"`
	OldLeafID           *string `json:"oldLeafId"`
	CommonAncestorID    *string `json:"commonAncestorId"`
	UserWantsSummary    bool    `json:"userWantsSummary"`
	CustomInstructions  string  `json:"customInstructions,omitempty"`
	ReplaceInstructions bool    `json:"replaceInstructions,omitempty"`
	Label               string  `json:"label,omitempty"`
}

// NavigateTreeResult contains the result of a tree navigation.
type NavigateTreeResult struct {
	Cancelled    bool                `json:"cancelled"`
	EditorText   string              `json:"editorText,omitempty"`
	SummaryEntry *BranchSummaryEntry `json:"summaryEntry,omitempty"`
}

// ============================================================================
// Miscellaneous
// ============================================================================

// FileOperations defines file operations for compaction.
type FileOperations struct {
	ReadTextFile  func(path string) Result[string, *FileError]
	WriteTextFile func(path string, content string) Result[struct{}, *FileError]
	Exists        func(path string) bool
}

// Ensure json is used
var _ = json.Marshal
var _ = time.Now
