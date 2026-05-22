package codecore

import (
	"fmt"
	"strings"
	"time"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/ai"
)

// ============================================================================
// Message Creation Helpers
// ============================================================================

// CreateUserMessage creates a user message with the given text.
func CreateUserMessage(text string) agent.AgentMessage {
	return agent.AgentMessage{
		Role:      ai.RoleUser,
		Content:   text,
		Timestamp: time.Now().UnixMilli(),
	}
}

// CreateAssistantMessage creates an assistant message.
func CreateAssistantMessage(text string, modelName string, usage *ai.Usage) agent.AgentMessage {
	msg := agent.AgentMessage{
		Role:      ai.RoleAssistant,
		Content:   text,
		Model:     modelName,
		Timestamp: time.Now().UnixMilli(),
	}
	if usage != nil {
		msg.Usage = usage
	}
	return msg
}

// CreateToolResultMessage creates a tool result message.
func CreateToolResultMessage(toolCallID, toolName, content string, isError bool) agent.AgentMessage {
	return agent.AgentMessage{
		Role:       ai.RoleToolResult,
		ToolCallID: toolCallID,
		ToolName:   toolName,
		Content:    content,
		IsError:    isError,
		Timestamp:  time.Now().UnixMilli(),
	}
}

// ============================================================================
// Formatting Helpers
// ============================================================================

// FormatMessageForDisplay formats a message for display in the TUI.
func FormatMessageForDisplay(msg agent.AgentMessage, width int) string {
	switch msg.Role {
	case ai.RoleUser:
		return "> " + msg.Content
	case ai.RoleAssistant:
		if len(msg.ToolCalls) > 0 {
			var parts []string
			for _, tc := range msg.ToolCalls {
				parts = append(parts, fmt.Sprintf("  🔧 %s(%v)", tc.Name, tc.Arguments))
			}
			return msg.Content + "\n" + strings.Join(parts, "\n")
		}
		return msg.Content
	case ai.RoleToolResult:
		content := msg.Content
		if width > 0 && len(content) > width {
			content = content[:width] + "..."
		}
		if msg.IsError {
			return fmt.Sprintf("  ⚠ [%s] Error: %s", msg.ToolName, content)
		}
		return fmt.Sprintf("  [%s] %s", msg.ToolName, content)
	}
	return msg.Content
}

// TruncateMessage truncates a message to a maximum length for previews.
func TruncateMessage(msg string, maxLen int) string {
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen] + "..."
}

// ============================================================================
// Custom Message Types
// ============================================================================

// CustomMessage represents a user-defined message type for extensions.
type CustomMessage struct {
	CustomType string      `json:"customType"`
	Content    string      `json:"content"`
	Display    interface{} `json:"display,omitempty"` // Custom display data
	Details    interface{} `json:"details,omitempty"`
}

// BashExecutionMessage represents a bash command execution in the session.
type BashExecutionMessage struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exitCode"`
	Output   string `json:"output"`
	Duration int64  `json:"duration"` // milliseconds
	Timeout  int    `json:"timeout,omitempty"`
	Error    string `json:"error,omitempty"`
}

// CompactionSummaryMessage represents a context compaction event.
type CompactionSummaryMessage struct {
	TokensBefore int    `json:"tokensBefore"`
	TokensAfter  int    `json:"tokensAfter"`
	Summary      string `json:"summary"`
	EntryCount   int    `json:"entryCount"`
}

// BranchSummaryMessage represents a branch summary in a session.
type BranchSummaryMessage struct {
	FromID     string `json:"fromId"`
	Summary    string `json:"summary"`
	EntryCount int    `json:"entryCount"`
	Label      string `json:"label,omitempty"`
}

// CreateCustomMessage creates a custom message from extension data.
func CreateCustomMessage(customType string, content string, display, details interface{}) *CustomMessage {
	return &CustomMessage{
		CustomType: customType,
		Content:    content,
		Display:    display,
		Details:    details,
	}
}

// CreateBashExecutionMessage creates a bash execution record.
func CreateBashExecutionMessage(cmd string, exitCode int, output string, duration int64) *BashExecutionMessage {
	return &BashExecutionMessage{
		Command:  cmd,
		ExitCode: exitCode,
		Output:   output,
		Duration: duration,
	}
}

// CreateCompactionSummaryMessage creates a compaction summary record.
func CreateCompactionSummaryMessage(tokensBefore, tokensAfter int, summary string, entryCount int) *CompactionSummaryMessage {
	return &CompactionSummaryMessage{
		TokensBefore: tokensBefore,
		TokensAfter:  tokensAfter,
		Summary:      summary,
		EntryCount:   entryCount,
	}
}

// CreateBranchSummaryMessage creates a branch summary record.
func CreateBranchSummaryMessage(fromID, summary string, entryCount int) *BranchSummaryMessage {
	return &BranchSummaryMessage{
		FromID:     fromID,
		Summary:    summary,
		EntryCount: entryCount,
	}
}

// ============================================================================
// Display Formatting
// ============================================================================

// FormatCustomMessage formats a custom message for display.
func FormatCustomMessage(cmsg *CustomMessage) string {
	if cmsg == nil {
		return ""
	}
	switch cmsg.CustomType {
	case "bash_execution":
		return fmt.Sprintf("[bash] %s", cmsg.Content)
	case "compaction":
		return fmt.Sprintf("[compaction] %s", TruncateMessage(cmsg.Content, 100))
	case "branch_summary":
		return fmt.Sprintf("[branch] %s", TruncateMessage(cmsg.Content, 100))
	default:
		return fmt.Sprintf("[%s] %s", cmsg.CustomType, TruncateMessage(cmsg.Content, 100))
	}
}

// FormatToolCall formats a tool call for display.
func FormatToolCall(tc ai.ToolCall) string {
	argsStr := formatArgs(tc.Arguments)
	if len(argsStr) > 60 {
		argsStr = argsStr[:60] + "..."
	}
	return fmt.Sprintf("🔧 %s(%s)", tc.Name, argsStr)
}

func formatArgs(args map[string]interface{}) string {
	if args == nil {
		return ""
	}
	var parts []string
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, ", ")
}
