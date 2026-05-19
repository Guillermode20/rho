package codecui

import (
	"fmt"
	"strings"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/tui"
)

// TreeNode represents a node in the session tree.
type TreeNode struct {
	ID        string
	Label     string
	Summary   string
	Timestamp string
	IsBranch  bool
	IsCurrent bool
	Children  []*TreeNode
	Depth     int
	Message   *agent.AgentMessage
}

// TreeSelectorComponent renders a session tree navigation overlay.
type TreeSelectorComponent struct {
	nodes         []*TreeNode
	flatNodes     []*TreeNode
	selectedIndex int
	maxVisible    int
	onSelect      func(node *TreeNode)
	onCancel      func()
	onNavigate    func(node *TreeNode)
	focused       bool
}

// NewTreeSelectorComponent creates a new tree selector.
func NewTreeSelectorComponent(rootNodes []*TreeNode, maxVisible int) *TreeSelectorComponent {
	s := &TreeSelectorComponent{
		maxVisible: maxVisible,
	}
	s.setNodes(rootNodes)
	return s
}

// setNodes flattens the tree for linear navigation.
func (s *TreeSelectorComponent) setNodes(nodes []*TreeNode) {
	s.nodes = nodes
	s.flatNodes = s.flattenTree(nodes, 0)
	if s.selectedIndex >= len(s.flatNodes) {
		s.selectedIndex = len(s.flatNodes) - 1
	}
}

// flattenTree converts tree to flat list with depth info.
func (s *TreeSelectorComponent) flattenTree(nodes []*TreeNode, depth int) []*TreeNode {
	var result []*TreeNode
	for _, node := range nodes {
		node.Depth = depth
		result = append(result, node)
		if len(node.Children) > 0 {
			result = append(result, s.flattenTree(node.Children, depth+1)...)
		}
	}
	return result
}

// SetOnSelect sets the select callback (jump to this node).
func (s *TreeSelectorComponent) SetOnSelect(fn func(node *TreeNode)) {
	s.onSelect = fn
}

// SetOnCancel sets the cancel callback.
func (s *TreeSelectorComponent) SetOnCancel(fn func()) {
	s.onCancel = fn
}

// SetOnNavigate sets the navigation callback (preview this node).
func (s *TreeSelectorComponent) SetOnNavigate(fn func(node *TreeNode)) {
	s.onNavigate = fn
}

func (s *TreeSelectorComponent) SetFocused(focused bool) { s.focused = focused }
func (s *TreeSelectorComponent) Focused() bool            { return s.focused }

// UpdateTree replaces the tree nodes.
func (s *TreeSelectorComponent) UpdateTree(nodes []*TreeNode) {
	s.setNodes(nodes)
}

func (s *TreeSelectorComponent) Render(width int) []string {
	if width <= 0 {
		return nil
	}

	dialogWidth := 70
	if width < dialogWidth {
		dialogWidth = width - 4
	}
	if dialogWidth < 40 {
		dialogWidth = 40
	}

	var lines []string
	lines = append(lines, "┌"+strings.Repeat("─", dialogWidth-2)+"┐")
	title := "  Session Tree History"
	lines = append(lines, fmt.Sprintf("│%s%s│", title, strings.Repeat(" ", dialogWidth-2-len(title))))
	lines = append(lines, "│"+strings.Repeat("─", dialogWidth-2)+"│")

	if len(s.flatNodes) == 0 {
		msg := "  No session history"
		lines = append(lines, fmt.Sprintf("│%s%s│", msg, strings.Repeat(" ", dialogWidth-2-len(msg))))
		lines = append(lines, "└"+strings.Repeat("─", dialogWidth-2)+"┘")
		return lines
	}

	// Calculate visible range
	total := len(s.flatNodes)
	startIdx := max(0, min(s.selectedIndex-s.maxVisible/2, total-s.maxVisible))
	endIdx := min(startIdx+s.maxVisible, total)

	for i := startIdx; i < endIdx; i++ {
		node := s.flatNodes[i]
		isSelected := i == s.selectedIndex

		// Build tree prefix with connectors
		prefix := ""
		if node.Depth > 0 {
			prefix += strings.Repeat("  ", node.Depth-1)
			if node.IsBranch {
				prefix += "├─ "
			} else {
				if i < total-1 && s.flatNodes[i+1].Depth > node.Depth {
					prefix += "│  "
				} else {
					prefix += "└─ "
				}
			}
		}

		sel := "  "
		if isSelected {
			sel = "▸ "
		}

		// Node marker
		marker := "○"
		if node.IsBranch {
			marker = "◇"
		}
		if node.IsCurrent {
			marker = "●"
		}

		markerColor := "\x1b[2m"
		if node.IsCurrent {
			markerColor = "\x1b[32m"
		} else if node.IsBranch {
			markerColor = "\x1b[36m"
		}

		label := node.Label
		if label == "" {
			label = "(root)"
		}

		// Truncate label
		maxLabelW := dialogWidth - 10 - len(prefix)
		if len(label) > maxLabelW {
			label = label[:maxLabelW-3] + "..."
		}

		// Timestamp
		ts := ""
		if node.Timestamp != "" {
			tsShort := node.Timestamp
			if len(tsShort) > 10 {
				tsShort = tsShort[:10]
			}
			ts = fmt.Sprintf(" \x1b[2m[%s]\x1b[0m", tsShort)
		}

		line := fmt.Sprintf("│%s%s%s%s %s%s%s│",
			sel,
			prefix,
			markerColor, marker, "\x1b[0m",
			label,
			ts,
		)
		lines = append(lines, line)
	}

	// Scroll indicator
	if endIdx < total {
		more := fmt.Sprintf("  ... %d more entries", total-endIdx)
		lines = append(lines, fmt.Sprintf("│%s%s│", more, strings.Repeat(" ", dialogWidth-2-len(more))))
	}

	// Hint
	hint := "  ↑↓ navigate · Enter jump · Esc cancel"
	lines = append(lines, "│"+strings.Repeat("─", dialogWidth-2)+"│")
	lines = append(lines, fmt.Sprintf("│%s%s│", hint, strings.Repeat(" ", dialogWidth-2-len(hint))))
	lines = append(lines, "└"+strings.Repeat("─", dialogWidth-2)+"┘")

	return lines
}

func (s *TreeSelectorComponent) HandleInput(data string) {
	switch {
	case tui.MatchesKey(data, "up") || tui.MatchesKey(data, "ctrl+p"):
		if s.selectedIndex > 0 {
			s.selectedIndex--
			s.notifyNavigate()
		}
	case tui.MatchesKey(data, "down") || tui.MatchesKey(data, "ctrl+n"):
		if s.selectedIndex < len(s.flatNodes)-1 {
			s.selectedIndex++
			s.notifyNavigate()
		}
	case tui.MatchesKey(data, "pageup"):
		s.selectedIndex -= s.maxVisible
		if s.selectedIndex < 0 {
			s.selectedIndex = 0
		}
		s.notifyNavigate()
	case tui.MatchesKey(data, "pagedown"):
		s.selectedIndex += s.maxVisible
		if s.selectedIndex >= len(s.flatNodes) {
			s.selectedIndex = len(s.flatNodes) - 1
		}
		s.notifyNavigate()
	case tui.MatchesKey(data, "home"):
		s.selectedIndex = 0
		s.notifyNavigate()
	case tui.MatchesKey(data, "end"):
		s.selectedIndex = len(s.flatNodes) - 1
		s.notifyNavigate()
	case tui.MatchesKey(data, "enter"):
		if s.selectedIndex < len(s.flatNodes) && s.onSelect != nil {
			s.onSelect(s.flatNodes[s.selectedIndex])
		}
	case tui.MatchesKey(data, "escape"):
		if s.onCancel != nil {
			s.onCancel()
		}
	}
}

func (s *TreeSelectorComponent) notifyNavigate() {
	if s.selectedIndex < len(s.flatNodes) && s.onNavigate != nil {
		s.onNavigate(s.flatNodes[s.selectedIndex])
	}
}

func (s *TreeSelectorComponent) Invalidate()          {}
func (s *TreeSelectorComponent) WantsKeyRelease() bool { return false }

// BuildTreeFromSession builds a tree from session entries.
func BuildTreeFromSession(messages []agent.AgentMessage) []*TreeNode {
	if len(messages) == 0 {
		return nil
	}

	var root *TreeNode
	var nodes []*TreeNode
	var currentBranch *TreeNode

	for i, msg := range messages {
		node := &TreeNode{
			ID:        fmt.Sprintf("msg_%d", i),
			Timestamp: fmt.Sprintf("%d", msg.Timestamp),
			Message:   &messages[i],
		}

		if msg.Content != "" {
			node.Label = msg.Content
			if len(node.Label) > 40 {
				node.Label = node.Label[:37] + "..."
			}
		} else if string(msg.Role) == "toolResult" {
			node.Label = fmt.Sprintf("[Tool: %s]", msg.ToolName)
		} else {
			node.Label = fmt.Sprintf("[%s message]", msg.Role)
		}

		if root == nil {
			root = node
			nodes = append(nodes, node)
			currentBranch = node
		} else {
			// Check if this is a branch point (first message after assistant)
			isUser := string(msg.Role) == "user"
			if currentBranch != nil && isUser {
				if currentBranch != root {
					node.IsBranch = true
				}
				currentBranch.Children = append(currentBranch.Children, node)
				currentBranch = node
			} else {
				currentBranch.Children = append(currentBranch.Children, node)
			}
		}

		nodes = append(nodes, node)
	}

	if root != nil {
		root.IsCurrent = true
	}

	return []*TreeNode{root}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ = agent.AgentMessage{}
