package tui

import (
	"sort"
	"strings"
)

// Keybinding describes a single keybinding action.
type Keybinding struct {
	Key         KeyID `json:"key"`
	Description string `json:"description"`
	Category    string `json:"category,omitempty"`
}

// KeybindingDefinition is a raw keybinding definition.
type KeybindingDefinition struct {
	Key         string `json:"key"`
	Command     string `json:"command"`
	Description string `json:"description"`
}

// KeybindingsConfig configures keybindings from a file.
type KeybindingsConfig struct {
	Keybindings []KeybindingDefinition `json:"keybindings"`
}

// KeybindingsManager manages keybinding registrations and conflicts.
type KeybindingsManager struct {
	bindings    map[KeyID][]Keybinding
	categories  map[string][]Keybinding
}

// NewKeybindingsManager creates a new keybindings manager.
func NewKeybindingsManager() *KeybindingsManager {
	return &KeybindingsManager{
		bindings:   make(map[KeyID][]Keybinding),
		categories: make(map[string][]Keybinding),
	}
}

// Register registers a keybinding.
func (km *KeybindingsManager) Register(kb Keybinding) {
	km.bindings[kb.Key] = append(km.bindings[kb.Key], kb)
	if kb.Category != "" {
		km.categories[kb.Category] = append(km.categories[kb.Category], kb)
	}
}

// RegisterMany registers multiple keybindings.
func (km *KeybindingsManager) RegisterMany(kbs []Keybinding) {
	for _, kb := range kbs {
		km.Register(kb)
	}
}

// Get returns all keybindings for a given key.
func (km *KeybindingsManager) Get(key KeyID) []Keybinding {
	return km.bindings[key]
}

// GetByCategory returns all keybindings in a category.
func (km *KeybindingsManager) GetByCategory(category string) []Keybinding {
	return km.categories[category]
}

// All returns all registered keybindings sorted by key.
func (km *KeybindingsManager) All() []Keybinding {
	var result []Keybinding
	for _, kbs := range km.bindings {
		result = append(result, kbs...)
	}
	sort.Slice(result, func(i, j int) bool {
		return string(result[i].Key) < string(result[j].Key)
	})
	return result
}

// Conflicts returns any conflicting keybindings.
func (km *KeybindingsManager) Conflicts() []KeybindingConflict {
	var conflicts []KeybindingConflict
	for key, kbs := range km.bindings {
		if len(kbs) > 1 {
			conflicts = append(conflicts, KeybindingConflict{
				Key:         key,
				Bindings:    kbs,
			})
		}
	}
	return conflicts
}

// KeybindingConflict describes conflicting keybindings.
type KeybindingConflict struct {
	Key      KeyID       `json:"key"`
	Bindings []Keybinding `json:"bindings"`
}

// FormatKeybinding formats a keybinding for display.
func FormatKeybinding(key KeyID) string {
	s := string(key)
	s = strings.ReplaceAll(s, "ctrl+", "⌃")
	s = strings.ReplaceAll(s, "shift+", "⇧")
	s = strings.ReplaceAll(s, "alt+", "⌥")
	s = strings.ReplaceAll(s, "meta+", "⌘")
	return s
}

// RenderKeybindingHints renders keybinding hints for display.
func RenderKeybindingHints(bindings []Keybinding, width int) string {
	if len(bindings) == 0 || width <= 0 {
		return ""
	}
	var parts []string
	for _, b := range bindings {
		hint := FormatKeybinding(b.Key) + " " + b.Description
		parts = append(parts, hint)
	}
	joined := strings.Join(parts, "  •  ")
	if VisibleWidth(joined) > width {
		joined = SliceByColumn(joined, 0, width, true)
	}
	return joined
}

// DefaultKeybindings returns the default set of keybindings.
func DefaultKeybindings() []Keybinding {
	return []Keybinding{
		{Key: "enter", Description: "Send", Category: "input"},
		{Key: "escape", Description: "Cancel", Category: "input"},
		{Key: "ctrl+c", Description: "Interrupt", Category: "global"},
		{Key: "ctrl+d", Description: "Debug", Category: "global"},
		{Key: "ctrl+l", Description: "Clear", Category: "global"},
		{Key: "tab", Description: "Focus next", Category: "navigation"},
		{Key: "shift+tab", Description: "Focus prev", Category: "navigation"},
		{Key: "ctrl+p", Description: "Previous", Category: "navigation"},
		{Key: "ctrl+n", Description: "Next", Category: "navigation"},
		{Key: "ctrl+r", Description: "Retry", Category: "session"},
		{Key: "ctrl+s", Description: "Save", Category: "session"},
		{Key: "ctrl+o", Description: "Open", Category: "session"},
		{Key: "ctrl+w", Description: "Close", Category: "session"},
		{Key: "ctrl+f", Description: "Find", Category: "search"},
		{Key: "ctrl+z", Description: "Undo", Category: "edit"},
		{Key: "ctrl+y", Description: "Redo", Category: "edit"},
	}
}
