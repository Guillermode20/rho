// Package codecui provides TUI components for the interactive coding agent mode.
package codecui

import (
	"fmt"
	"strings"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/ai"
	"github.com/earendil-works/rho/pkg/tui"
)

// AssistantMessage displays an assistant message with text, thinking, and tool calls.
type AssistantMessage struct {
	msg       agent.AgentMessage
	theme     tui.MarkdownTheme
	expanded  bool
	showThinking bool
}

// NewAssistantMessage creates a new assistant message component.
func NewAssistantMessage(msg agent.AgentMessage) *AssistantMessage {
	return &AssistantMessage{
		msg:       msg,
		theme:     tui.DefaultMarkdownTheme(),
		expanded:  true,
		showThinking: false,
	}
}

// SetExpanded sets whether the message is expanded.
func (am *AssistantMessage) SetExpanded(expanded bool) {
	am.expanded = expanded
}

// SetShowThinking sets whether thinking blocks are shown.
func (am *AssistantMessage) SetShowThinking(show bool) {
	am.showThinking = show
}

func (am *AssistantMessage) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	var lines []string
	theme := am.theme
	reset := "\x1b[0m"

	// Model name header
	modelName := am.msg.Model
	if modelName == "" {
		modelName = "Assistant"
	}

	// Usage info if available
	usageStr := ""
	if am.msg.Usage != nil {
		usageStr = fmt.Sprintf(" [%d↑ %d↓]", am.msg.Usage.Input, am.msg.Usage.Output)
	}
	header := theme.H2(modelName+usageStr) + reset
	lines = append(lines, header)

	if !am.expanded {
		lines = append(lines, theme.H3("  (collapsed)")+reset)
		return lines
	}

	// Process content blocks if we have structured content
	if len(am.msg.ToolCalls) > 0 || strings.Contains(am.msg.Content, "toolCall") {
		// Parse out text and thinking from content
		text := am.msg.Content

		// Show text content
		if text != "" {
			md := tui.NewMarkdown(text, theme)
			contentLines := md.Render(width - 4)
			for _, l := range contentLines {
				if l != "" {
					lines = append(lines, "  "+l)
				}
			}
		}

		// Show tool calls
		for _, tc := range am.msg.ToolCalls {
			argsJSON := fmt.Sprintf("%v", tc.Arguments)
			callLine := fmt.Sprintf("  🔧 %s(%s)", tc.Name, argsJSON)
			lines = append(lines, theme.Code(callLine)+reset)
		}
	} else {
		// Simple text content
		if am.msg.Content != "" {
			md := tui.NewMarkdown(am.msg.Content, theme)
			contentLines := md.Render(width - 4)
			for _, l := range contentLines {
				if l != "" {
					lines = append(lines, "  "+l)
				}
			}
		}
	}

	// Error state
	if am.msg.ErrorMessage != "" {
		lines = append(lines, "")
		lines = append(lines, "  ⚠ "+theme.Code("Error: "+am.msg.ErrorMessage)+reset)
	}

	// Stop reason
	if am.msg.StopReason != "" && am.msg.StopReason != ai.StopReasonStop {
		lines = append(lines, "")
		lines = append(lines, "  ["+theme.Code(string(am.msg.StopReason))+"]"+reset)
	}

	return lines
}

func (am *AssistantMessage) HandleInput(data string) {
	if tui.MatchesKey(data, "enter") {
		am.expanded = !am.expanded
	}
	if tui.MatchesKey(data, "t") {
		am.showThinking = !am.showThinking
	}
}

func (am *AssistantMessage) Invalidate() {}

func (am *AssistantMessage) WantsKeyRelease() bool { return false }

// CompactionSummaryMessage displays a compaction summary.
type CompactionSummaryMessage struct {
	summary string
	tokensBefore int
	theme tui.MarkdownTheme
}

// NewCompactionSummaryMessage creates a new compaction summary component.
func NewCompactionSummaryMessage(summary string, tokensBefore int) *CompactionSummaryMessage {
	return &CompactionSummaryMessage{
		summary: summary,
		tokensBefore: tokensBefore,
		theme: tui.DefaultMarkdownTheme(),
	}
}

func (cs *CompactionSummaryMessage) Render(width int) []string {
	if width <= 0 {
		return nil
	}
	reset := "\x1b[0m"
	dim := "\x1b[2m"

	var lines []string
	lines = append(lines, dim+"┌─ Context Compaction ──────────────────────┐"+reset)
	lines = append(lines, dim+"│"+reset)
	lines = append(lines, fmt.Sprintf(dim+"│  Tokens before: %d"+reset, cs.tokensBefore))
	if cs.summary != "" {
		md := tui.NewMarkdown(cs.summary, cs.theme)
		summaryLines := md.Render(width - 6)
		for _, l := range summaryLines {
			if l != "" {
				lines = append(lines, dim+"│  "+l+reset)
			}
		}
	}
	lines = append(lines, dim+"│"+reset)
	lines = append(lines, dim+"└───────────────────────────────────────────┘"+reset)
	return lines
}

func (cs *CompactionSummaryMessage) HandleInput(data string) {}
func (cs *CompactionSummaryMessage) Invalidate()            {}
func (cs *CompactionSummaryMessage) WantsKeyRelease() bool  { return false }

// BranchSummaryMessage displays a branch summary in the session tree.
type BranchSummaryMessage struct {
	fromID  string
	summary string
	label   string
	theme   tui.MarkdownTheme
}

// NewBranchSummaryMessage creates a new branch summary component.
func NewBranchSummaryMessage(fromID, summary, label string) *BranchSummaryMessage {
	return &BranchSummaryMessage{
		fromID:  fromID,
		summary: summary,
		label:   label,
		theme:   tui.DefaultMarkdownTheme(),
	}
}

func (bs *BranchSummaryMessage) Render(width int) []string {
	if width <= 0 {
		return nil
	}
	reset := "\x1b[0m"
	dim := "\x1b[2m"
	cyan := "\x1b[36m"

	var lines []string
	lines = append(lines, dim+"┌─ Branch Summary"+reset)
	if bs.label != "" {
		lines = append(lines, dim+"│  Label: "+cyan+bs.label+reset)
	}
	if bs.summary != "" {
		md := tui.NewMarkdown(bs.summary, bs.theme)
		summaryLines := md.Render(width - 6)
		for _, l := range summaryLines {
			if l != "" {
				lines = append(lines, dim+"│  "+l+reset)
			}
		}
	}
	lines = append(lines, dim+"└───────────────────────────────────────────┘"+reset)
	return lines
}

func (bs *BranchSummaryMessage) HandleInput(data string) {}
func (bs *BranchSummaryMessage) Invalidate()             {}
func (bs *BranchSummaryMessage) WantsKeyRelease() bool   { return false }
