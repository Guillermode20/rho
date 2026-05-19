package codecui

import (
	"fmt"
	"strings"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/ai"
	"github.com/earendil-works/rho/pkg/tui"
)

// UserMessage displays a user message with text and optional images.
type UserMessage struct {
	msg   agent.AgentMessage
	theme tui.MarkdownTheme
}

// NewUserMessage creates a new user message component.
func NewUserMessage(msg agent.AgentMessage) *UserMessage {
	return &UserMessage{
		msg:   msg,
		theme: tui.DefaultMarkdownTheme(),
	}
}

func (um *UserMessage) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	var lines []string
	theme := um.theme
	reset := "\x1b[0m"
	dim := "\x1b[2m"

	// User header
	lines = append(lines, theme.H2("You:")+reset)

	if um.msg.Content != "" {
		md := tui.NewMarkdown(um.msg.Content, theme)
		contentLines := md.Render(width - 4)
		for _, l := range contentLines {
			if l != "" {
				lines = append(lines, "  "+l)
			}
		}
	}

	// Images (placeholder)
	if um.msg.Content != "" && strings.Contains(um.msg.Content, "[image:") {
		lines = append(lines, dim+"  [Image attached]"+reset)
	}

	return lines
}

func (um *UserMessage) HandleInput(data string) {}
func (um *UserMessage) Invalidate()            {}
func (um *UserMessage) WantsKeyRelease() bool  { return false }

// UserMessageSelector allows selecting from multiple user messages.
type UserMessageSelector struct {
	messages []agent.AgentMessage
	selected int
	onSelect func(msg agent.AgentMessage)
	theme    tui.MarkdownTheme
}

// NewUserMessageSelector creates a new user message selector.
func NewUserMessageSelector(messages []agent.AgentMessage, onSelect func(msg agent.AgentMessage)) *UserMessageSelector {
	return &UserMessageSelector{
		messages: messages,
		selected: 0,
		onSelect: onSelect,
		theme:    tui.DefaultMarkdownTheme(),
	}
}

func (ums *UserMessageSelector) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	var lines []string
	theme := ums.theme
	reset := "\x1b[0m"
	cyan := "\x1b[36m"
	dim := "\x1b[2m"

	lines = append(lines, theme.H2("Select Message to Edit/Resend:")+reset)

	for i, msg := range ums.messages {
		if i >= 10 { // limit display
			lines = append(lines, dim+"  ..."+reset)
			break
		}

		prefix := "  "
		if i == ums.selected {
			prefix = "> " + cyan
		}

		content := msg.Content
		if len(content) > 60 {
			content = content[:60] + "..."
		}
		content = strings.ReplaceAll(content, "\n", " ")

		lines = append(lines, fmt.Sprintf("%s%d. %s", prefix, i+1, content))
	}

	return lines
}

func (ums *UserMessageSelector) HandleInput(data string) {
	switch {
	case tui.MatchesKey(data, "up") || tui.MatchesKey(data, "ctrl+p"):
		if ums.selected > 0 {
			ums.selected--
		}
	case tui.MatchesKey(data, "down") || tui.MatchesKey(data, "ctrl+n"):
		if ums.selected < len(ums.messages)-1 {
			ums.selected++
		}
	case tui.MatchesKey(data, "enter"):
		if ums.selected < len(ums.messages) && ums.onSelect != nil {
			ums.onSelect(ums.messages[ums.selected])
		}
	}
}

func (ums *UserMessageSelector) Invalidate()            {}
func (ums *UserMessageSelector) WantsKeyRelease() bool  { return false }

// Ensure imports are used
var _ = ai.RoleUser
