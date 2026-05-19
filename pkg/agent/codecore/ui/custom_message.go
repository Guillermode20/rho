package codecui

import (
	"strings"
	"sync"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/tui"
)

// MessageRendererFunc renders a custom agent message to TUI lines.
type MessageRendererFunc func(msg agent.AgentMessage, width int) []string

var (
	mu               sync.Mutex
	messageRenderers []MessageRendererFunc
)

// RegisterMessageRenderer registers a custom message renderer.
func RegisterMessageRenderer(renderer MessageRendererFunc) {
	mu.Lock()
	defer mu.Unlock()
	messageRenderers = append(messageRenderers, renderer)
}

// CustomMessageComponent renders a custom agent message by dispatching to
// registered extension renderers. Falls back to generic display for unknown types.
type CustomMessageComponent struct {
	message agent.AgentMessage
	customType string
	width     int
}

// NewCustomMessageComponent creates a new custom message component.
func NewCustomMessageComponent(msg agent.AgentMessage, customType string) *CustomMessageComponent {
	return &CustomMessageComponent{
		message:    msg,
		customType: customType,
	}
}

func (c *CustomMessageComponent) Render(width int) []string {
	if width <= 0 {
		return nil
	}
	c.width = width

	mu.Lock()
	renderers := make([]MessageRendererFunc, len(messageRenderers))
	copy(renderers, messageRenderers)
	mu.Unlock()

	// Try registered renderers first
	for _, renderer := range renderers {
		if lines := renderer(c.message, width); lines != nil {
			return lines
		}
	}

	// Fallback: generic display
	return c.genericRender()
}

func (c *CustomMessageComponent) genericRender() []string {
	var lines []string

	// Header
	lines = append(lines, "\x1b[1;35m┌─ "+c.customType+" ───────────────────────┐\x1b[0m")

	// Content
	content := c.message.Content
	if content == "" {
		content = "(no content)"
	}
	contentLines := strings.Split(content, "\n")
	for _, l := range contentLines {
		truncated := l
		if len(truncated) > c.width-6 {
			truncated = truncated[:c.width-9] + "..."
		}
		lines = append(lines, "\x1b[35m│\x1b[0m "+truncated)
	}

	// Footer
	lines = append(lines, "\x1b[1;35m└──────────────────────────────────────┘\x1b[0m")

	return lines
}

func (c *CustomMessageComponent) HandleInput(data string) {}
func (c *CustomMessageComponent) Invalidate()            {}
func (c *CustomMessageComponent) WantsKeyRelease() bool  { return false }

// GenericMessageComponent renders any agent message in a standardized format.
// Used for messages that don't have a specific custom renderer.
type GenericMessageComponent struct {
	message agent.AgentMessage
}

// NewGenericMessageComponent creates a generic message component.
func NewGenericMessageComponent(msg agent.AgentMessage) *GenericMessageComponent {
	return &GenericMessageComponent{message: msg}
}

func (g *GenericMessageComponent) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	var lines []string

	switch g.message.Role {
	case "user":
		lines = append(lines, "\x1b[1;36m┌─ You ──────────────────────────────────┐\x1b[0m")
	case "assistant":
		modelName := g.message.Model
		if modelName == "" {
			modelName = "Assistant"
		}
		lines = append(lines, "\x1b[1;32m┌─ "+modelName+" ───────────────────────────┐\x1b[0m")
	case "toolResult":
		lines = append(lines, "\x1b[1;33m┌─ Tool: "+g.message.ToolName+" ───────────────────┐\x1b[0m")
	default:
		lines = append(lines, "\x1b[1;37m┌─ Message ───────────────────────────────┐\x1b[0m")
	}

	// Content
	if g.message.Content != "" {
		contentLines := strings.Split(g.message.Content, "\n")
		for _, l := range contentLines {
			truncated := l
			if len(truncated) > width-8 {
				truncated = truncated[:width-11] + "..."
			}
			lines = append(lines, "│ "+truncated)
		}
	}

	// Error
	if g.message.ErrorMessage != "" {
		lines = append(lines, "\x1b[31m│ Error: "+g.message.ErrorMessage+"\x1b[0m")
	}

	// Tool calls
	if len(g.message.ToolCalls) > 0 {
		for _, tc := range g.message.ToolCalls {
			lines = append(lines, "│ \x1b[36m🔧 "+tc.Name+"\x1b[0m")
		}
	}

	lines = append(lines, "└────────────────────────────────────────────┘")
	return lines
}

func (g *GenericMessageComponent) HandleInput(data string) {}
func (g *GenericMessageComponent) Invalidate()             {}
func (g *GenericMessageComponent) WantsKeyRelease() bool   { return false }

var _ = tui.MarkdownTheme{}
