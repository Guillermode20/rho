package harness

import (
	"fmt"
	"strings"
	"time"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/ai"
)

// CustomMessageType defines a custom message type for the harness.
type CustomMessageType string

const (
	MsgTypeBashExecution        CustomMessageType = "bash_execution"
	MsgTypeCompactionSummary    CustomMessageType = "compaction_summary"
	MsgTypeBranchSummary        CustomMessageType = "branch_summary"
	MsgTypeSkillInvocation      CustomMessageType = "skill_invocation"
	MsgTypeCustomNotification   CustomMessageType = "custom_notification"
)

// CreateBashExecutionMessage creates a bash execution message for the session.
func CreateBashExecutionMessage(command, output string, exitCode int, isError bool) agent.AgentMessage {
	return agent.AgentMessage{
		Role:      ai.RoleToolResult,
		ToolName:  "Bash",
		Content:   output,
		IsError:   isError,
		Timestamp: time.Now().UnixMilli(),
		Hide:      false,
	}
}

// CreateCompactionSummaryMessage creates a compaction summary message.
func CreateCompactionSummaryMessage(summary string, tokensBefore int, fromHook bool) agent.AgentMessage {
	return agent.AgentMessage{
		Role:    ai.RoleAssistant,
		Content: formatCompactionSummary(summary, tokensBefore),
		Model:   "system",
		Hide:    true,
		Timestamp: time.Now().UnixMilli(),
	}
}

func formatCompactionSummary(summary string, tokensBefore int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Context window was at %d tokens. Older messages were compacted:\n\n", tokensBefore))
	b.WriteString(summary)
	return b.String()
}

// CreateBranchSummaryMessage creates a branch summary message for display.
func CreateBranchSummaryMessage(summary string, fromEntryID string) agent.AgentMessage {
	return agent.AgentMessage{
		Role:    ai.RoleAssistant,
		Content: fmt.Sprintf("Branch resumed from a previous point.\n\n**Summary of previous context:**\n%s", summary),
		Model:   "system",
		Hide:    true,
		Timestamp: time.Now().UnixMilli(),
	}
}

// CreateSkillInvocationMessage creates a skill invocation message.
func CreateSkillInvocationMessage(skillName, skillContent, additionalInstructions string) agent.AgentMessage {
	content := fmt.Sprintf("Invoking skill: %s\n\n%s", skillName, skillContent)
	if additionalInstructions != "" {
		content += "\n\nAdditional instructions:\n" + additionalInstructions
	}
	return agent.AgentMessage{
		Role:    ai.RoleUser,
		Content: content,
		Timestamp: time.Now().UnixMilli(),
	}
}

// CreateCustomMessage creates a custom message with arbitrary type and content.
func CreateCustomMessage(customType string, content string, details interface{}, display bool) agent.AgentMessage {
	msg := agent.AgentMessage{
		Role:    ai.RoleAssistant,
		Content: content,
		Model:   fmt.Sprintf("custom:%s", customType),
		Hide:    !display,
		Timestamp: time.Now().UnixMilli(),
	}
	return msg
}

// FormatToolResult formats a tool result as a string.
func FormatToolResult(toolName string, content string, isError bool) string {
	prefix := "✅"
	if isError {
		prefix = "❌"
	}
	return fmt.Sprintf("%s %s: %s", prefix, toolName, strings.TrimSpace(content))
}

// FormatAgentMessage formats an AgentMessage as a human-readable string.
func FormatAgentMessage(msg agent.AgentMessage) string {
	switch msg.Role {
	case ai.RoleUser:
		return fmt.Sprintf("User: %s", msg.Content)
	case ai.RoleAssistant:
		if len(msg.ToolCalls) > 0 {
			var tcs []string
			for _, tc := range msg.ToolCalls {
				tcs = append(tcs, fmt.Sprintf("  🔧 %s(%v)", tc.Name, tc.Arguments))
			}
			result := ""
			if msg.Content != "" {
				result = fmt.Sprintf("Assistant: %s\n", msg.Content)
			}
			result += strings.Join(tcs, "\n")
			return result
		}
		return fmt.Sprintf("Assistant: %s", msg.Content)
	case ai.RoleToolResult:
		return FormatToolResult(msg.ToolName, msg.Content, msg.IsError)
	default:
		return fmt.Sprintf("[%s] %s", msg.Role, msg.Content)
	}
}

// IsCustomMessage checks if a message has a custom type.
func IsCustomMessage(msg agent.AgentMessage) bool {
	return strings.HasPrefix(msg.Model, "custom:")
}

// GetCustomMessageType extracts the custom type from a custom message.
func GetCustomMessageType(msg agent.AgentMessage) string {
	if !IsCustomMessage(msg) {
		return ""
	}
	return strings.TrimPrefix(msg.Model, "custom:")
}
