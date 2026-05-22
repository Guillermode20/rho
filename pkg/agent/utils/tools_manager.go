package agentutils

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/earendil-works/rho/pkg/agent"
)

// ToolStatus represents the state of a tool.
type ToolStatus string

const (
	ToolEnabled  ToolStatus = "enabled"
	ToolDisabled ToolStatus = "disabled"
)

// ToolInfo holds metadata about a registered tool.
type ToolInfo struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Status      ToolStatus `json:"status"`
	Source      string     `json:"source"` // "builtin", "extension"
	Order       int        `json:"order"`
}

// ToolsManager manages tool lifecycle: enable/disable, ordering, status.
type ToolsManager struct {
	mu    sync.RWMutex
	tools map[string]*ToolInfo
	order []string // ordered list of tool names
}

// NewToolsManager creates a new tools manager.
func NewToolsManager() *ToolsManager {
	return &ToolsManager{
		tools: make(map[string]*ToolInfo),
	}
}

// Register adds a tool to the manager.
func (tm *ToolsManager) Register(name, description, source string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if _, exists := tm.tools[name]; exists {
		return
	}
	tm.tools[name] = &ToolInfo{
		Name:        name,
		Description: description,
		Status:      ToolEnabled,
		Source:      source,
		Order:       len(tm.order),
	}
	tm.order = append(tm.order, name)
}

// Enable enables a tool.
func (tm *ToolsManager) Enable(name string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	t, ok := tm.tools[name]
	if !ok {
		return fmt.Errorf("unknown tool: %s", name)
	}
	t.Status = ToolEnabled
	return nil
}

// Disable disables a tool.
func (tm *ToolsManager) Disable(name string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	t, ok := tm.tools[name]
	if !ok {
		return fmt.Errorf("unknown tool: %s", name)
	}
	t.Status = ToolDisabled
	return nil
}

// IsEnabled checks if a tool is enabled.
func (tm *ToolsManager) IsEnabled(name string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	t, ok := tm.tools[name]
	if !ok {
		return false
	}
	return t.Status == ToolEnabled
}

// GetStatus returns the status of a tool.
func (tm *ToolsManager) GetStatus(name string) (ToolStatus, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	t, ok := tm.tools[name]
	if !ok {
		return "", false
	}
	return t.Status, true
}

// GetOrdered returns all enabled tools in registration order.
func (tm *ToolsManager) GetOrdered() []ToolInfo {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	var result []ToolInfo
	for _, name := range tm.order {
		if t, ok := tm.tools[name]; ok && t.Status == ToolEnabled {
			result = append(result, *t)
		}
	}
	return result
}

// GetAll returns all tools regardless of status.
func (tm *ToolsManager) GetAll() []ToolInfo {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	var result []ToolInfo
	for _, name := range tm.order {
		if t, ok := tm.tools[name]; ok {
			result = append(result, *t)
		}
	}
	return result
}

// SetOrder sets the tool ordering by name.
func (tm *ToolsManager) SetOrder(names []string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	// Validate all names exist
	for _, name := range names {
		if _, ok := tm.tools[name]; !ok {
			return fmt.Errorf("unknown tool: %s", name)
		}
	}
	tm.order = make([]string, len(names))
	copy(tm.order, names)
	// Update order field
	for i, name := range tm.order {
		if t, ok := tm.tools[name]; ok {
			t.Order = i
		}
	}
	return nil
}

// FilterTools filters a list of agent tools based on the manager's enabled state.
func (tm *ToolsManager) FilterTools(tools []agent.AgentTool) []agent.AgentTool {
	var filtered []agent.AgentTool
	for _, tool := range tools {
		if tm.IsEnabled(tool.Name) {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

// SortTools sorts tools according to the manager's ordering.
func (tm *ToolsManager) SortTools(tools []agent.AgentTool) {
	tm.mu.RLock()
	order := make(map[string]int)
	for i, name := range tm.order {
		order[name] = i
	}
	tm.mu.RUnlock()

	sort.Slice(tools, func(i, j int) bool {
		oi, oki := order[tools[i].Name]
		oj, okj := order[tools[j].Name]
		if !oki {
			oi = 999
		}
		if !okj {
			oj = 999
		}
		return oi < oj
	})
}

// String returns a formatted summary of all tools.
func (tm *ToolsManager) String() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	var b strings.Builder
	b.WriteString("Tools:\n")
	for _, name := range tm.order {
		if t, ok := tm.tools[name]; ok {
			status := "✓"
			if t.Status == ToolDisabled {
				status = "✗"
			}
			b.WriteString(fmt.Sprintf("  %s %s: %s\n", status, name, t.Description))
		}
	}
	return b.String()
}
