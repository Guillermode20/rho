package codecore

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/earendil-works/rho/pkg/tui"
)

// AppKeybinding is a named keybinding used throughout the application.
type AppKeybinding struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Description string `json:"description"`
	Category    string `json:"category"`
	DefaultKey  string `json:"defaultKey"`
}

// ReservedKeybindingIDs are the app-level keybinding IDs that extensions cannot override.
var ReservedKeybindingIDs = []string{
	"app.interrupt",
	"app.clear",
	"app.exit",
	"app.suspend",
	"app.thinking.cycle",
	"app.model.cycleForward",
	"app.model.cycleBackward",
	"app.model.select",
	"app.tools.expand",
	"app.thinking.toggle",
	"app.editor.external",
	"app.message.followUp",
	"tui.input.submit",
	"tui.select.confirm",
	"tui.select.cancel",
	"tui.input.copy",
	"tui.editor.deleteToLineEnd",
}

// DefaultKeybindings returns the default app keybindings.
func DefaultKeybindings() []AppKeybinding {
	return []AppKeybinding{
		{ID: "app.interrupt", Key: "ctrl+c", DefaultKey: "ctrl+c", Description: "Interrupt current operation", Category: "app"},
		{ID: "app.clear", Key: "ctrl+l", DefaultKey: "ctrl+l", Description: "Clear screen", Category: "app"},
		{ID: "app.exit", Key: "ctrl+d", DefaultKey: "ctrl+d", Description: "Exit pi", Category: "app"},
		{ID: "app.suspend", Key: "ctrl+z", DefaultKey: "ctrl+z", Description: "Suspend pi", Category: "app"},
		{ID: "app.thinking.cycle", Key: "ctrl+shift+t", DefaultKey: "ctrl+shift+t", Description: "Cycle thinking level", Category: "model"},
		{ID: "app.model.cycleForward", Key: "ctrl+shift+m", DefaultKey: "ctrl+shift+m", Description: "Next model", Category: "model"},
		{ID: "app.model.cycleBackward", Key: "ctrl+shift+n", DefaultKey: "ctrl+shift+n", Description: "Previous model", Category: "model"},
		{ID: "app.model.select", Key: "ctrl+shift+p", DefaultKey: "ctrl+shift+p", Description: "Select model", Category: "model"},
		{ID: "app.tools.expand", Key: "ctrl+e", DefaultKey: "ctrl+e", Description: "Toggle tool output", Category: "tools"},
		{ID: "app.thinking.toggle", Key: "ctrl+shift+l", DefaultKey: "ctrl+shift+l", Description: "Toggle thinking display", Category: "model"},
		{ID: "app.editor.external", Key: "ctrl+shift+e", DefaultKey: "ctrl+shift+e", Description: "Open in external editor", Category: "editor"},
		{ID: "app.message.followUp", Key: "ctrl+r", DefaultKey: "ctrl+r", Description: "Send follow-up message", Category: "messages"},
		{ID: "tui.input.submit", Key: "enter", DefaultKey: "enter", Description: "Submit input", Category: "input"},
		{ID: "tui.select.confirm", Key: "enter", DefaultKey: "enter", Description: "Confirm selection", Category: "select"},
		{ID: "tui.select.cancel", Key: "escape", DefaultKey: "escape", Description: "Cancel selection", Category: "select"},
		{ID: "tui.input.copy", Key: "ctrl+shift+c", DefaultKey: "ctrl+shift+c", Description: "Copy selection", Category: "input"},
		{ID: "tui.editor.deleteToLineEnd", Key: "ctrl+k", DefaultKey: "ctrl+k", Description: "Delete to end of line", Category: "editor"},
	}
}

// KeybindingsConfig represents a keybindings configuration file.
type KeybindingsConfig struct {
	Version     int                  `json:"version"`
	Keybindings []KeybindingOverride `json:"keybindings"`
}

// KeybindingOverride overrides a keybinding.
type KeybindingOverride struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

// KeybindingsManager manages keybinding resolution with overrides.
type KeybindingsManager struct {
	mu        sync.RWMutex
	bindings  map[string]*AppKeybinding   // ID -> binding (with current key)
	defaults  map[string]*AppKeybinding   // ID -> default binding
	byKey     map[string][]*AppKeybinding // key -> bindings (for conflict detection)
	overrides map[string]string           // ID -> overridden key
}

// NewKeybindingsManager creates a new KeybindingsManager.
func NewKeybindingsManager() *KeybindingsManager {
	km := &KeybindingsManager{
		bindings:  make(map[string]*AppKeybinding),
		defaults:  make(map[string]*AppKeybinding),
		byKey:     make(map[string][]*AppKeybinding),
		overrides: make(map[string]string),
	}
	for _, kb := range DefaultKeybindings() {
		b := &AppKeybinding{
			ID: kb.ID, Key: kb.Key, Description: kb.Description,
			Category: kb.Category, DefaultKey: kb.DefaultKey,
		}
		km.defaults[kb.ID] = b
		km.bindings[kb.ID] = b
		km.byKey[kb.Key] = append(km.byKey[kb.Key], b)
	}
	return km
}

// GetBinding returns the binding for a given ID.
func (km *KeybindingsManager) GetBinding(id string) *AppKeybinding {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.bindings[id]
}

// GetKey returns the current key for a binding ID.
func (km *KeybindingsManager) GetKey(id string) string {
	km.mu.RLock()
	defer km.mu.RUnlock()
	if b, ok := km.bindings[id]; ok {
		return b.Key
	}
	return ""
}

// SetOverride sets a keybinding override.
func (km *KeybindingsManager) SetOverride(id, newKey string) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	// Check if this is a reserved binding
	isReserved := false
	for _, rid := range ReservedKeybindingIDs {
		if rid == id {
			isReserved = true
			break
		}
	}

	// Find the default
	def, hasDefault := km.defaults[id]
	if !hasDefault {
		return fmt.Errorf("unknown keybinding: %s", id)
	}

	// Remove old key mapping
	oldKey := km.bindings[id].Key
	oldBindings := km.byKey[oldKey]
	for i, b := range oldBindings {
		if b.ID == id {
			km.byKey[oldKey] = append(oldBindings[:i], oldBindings[i+1:]...)
			break
		}
	}

	// Create binding with new key
	b := &AppKeybinding{
		ID:          def.ID,
		Key:         newKey,
		Description: def.Description,
		Category:    def.Category,
		DefaultKey:  def.DefaultKey,
	}
	km.bindings[id] = b
	km.overrides[id] = newKey
	km.byKey[newKey] = append(km.byKey[newKey], b)

	if !isReserved {
		_ = isReserved // Non-reserved bindings can be overridden freely
	}
	return nil
}

// ResetBinding resets a binding to its default key.
func (km *KeybindingsManager) ResetBinding(id string) {
	km.mu.Lock()
	defer km.mu.Unlock()
	if def, ok := km.defaults[id]; ok {
		oldKey := km.bindings[id].Key
		oldBindings := km.byKey[oldKey]
		for i, b := range oldBindings {
			if b.ID == id {
				km.byKey[oldKey] = append(oldBindings[:i], oldBindings[i+1:]...)
				break
			}
		}
		km.bindings[id] = def
		delete(km.overrides, id)
		km.byKey[def.Key] = append(km.byKey[def.Key], def)
	}
}

// GetConflicts returns any conflicting keybindings.
func (km *KeybindingsManager) GetConflicts() []KeybindingConflict {
	km.mu.RLock()
	defer km.mu.RUnlock()
	var conflicts []KeybindingConflict
	for key, bindings := range km.byKey {
		if len(bindings) > 1 {
			conflicts = append(conflicts, KeybindingConflict{
				Key:      tui.KeyID(key),
				Bindings: bindings,
			})
		}
	}
	sort.Slice(conflicts, func(i, j int) bool {
		return string(conflicts[i].Key) < string(conflicts[j].Key)
	})
	return conflicts
}

// KeybindingConflict describes a keybinding conflict.
type KeybindingConflict struct {
	Key      tui.KeyID        `json:"key"`
	Bindings []*AppKeybinding `json:"bindings"`
}

// LoadBindingsFromFile loads keybinding overrides from a JSON file.
func (km *KeybindingsManager) LoadBindingsFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot read keybindings file: %w", err)
	}

	var cfg KeybindingsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("cannot parse keybindings file: %w", err)
	}

	for _, ov := range cfg.Keybindings {
		if err := km.SetOverride(ov.ID, ov.Key); err != nil {
			return fmt.Errorf("keybinding override error for %q: %w", ov.ID, err)
		}
	}
	return nil
}

// SaveBindingsToFile saves current keybinding overrides to a JSON file.
func (km *KeybindingsManager) SaveBindingsToFile(path string) error {
	km.mu.RLock()
	defer km.mu.RUnlock()

	var overrides []KeybindingOverride
	for id, key := range km.overrides {
		overrides = append(overrides, KeybindingOverride{ID: id, Key: key})
	}
	sort.Slice(overrides, func(i, j int) bool { return overrides[i].ID < overrides[j].ID })

	cfg := KeybindingsConfig{
		Version:     1,
		Keybindings: overrides,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal keybindings: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// IsReservedBinding checks if a binding ID is reserved.
func IsReservedBinding(id string) bool {
	for _, rid := range ReservedKeybindingIDs {
		if rid == id {
			return true
		}
	}
	return false
}

// FormatKeybinding formats a keybinding for display (e.g., "ctrl+c" → "⌃C").
func FormatKeybinding(key string) string {
	s := key
	s = strings.ReplaceAll(s, "ctrl+", "⌃")
	s = strings.ReplaceAll(s, "shift+", "⇧")
	s = strings.ReplaceAll(s, "alt+", "⌥")
	s = strings.ReplaceAll(s, "meta+", "⌘")
	s = strings.ReplaceAll(s, "escape", "Esc")
	s = strings.ReplaceAll(s, "enter", "↵")
	s = strings.ReplaceAll(s, "tab", "⇥")
	s = strings.ReplaceAll(s, "backspace", "⌫")
	s = strings.ReplaceAll(s, "up", "↑")
	s = strings.ReplaceAll(s, "down", "↓")
	s = strings.ReplaceAll(s, "left", "←")
	s = strings.ReplaceAll(s, "right", "→")
	s = strings.ReplaceAll(s, "pageup", "⇞")
	s = strings.ReplaceAll(s, "pagedown", "⇟")
	s = strings.ReplaceAll(s, "home", "↖")
	s = strings.ReplaceAll(s, "end", "↘")
	return s
}
