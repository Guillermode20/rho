// Package theme provides a theme system for the TUI with loadable JSON theme files.
package theme

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ThemeColor is a single color definition (named or ANSI code).
type ThemeColor struct {
	Name      string `json:"name,omitempty"`
	ANSI      int    `json:"ansi,omitempty"`
	Hex       string `json:"hex,omitempty"`
	Bold      bool   `json:"bold,omitempty"`
	Italic    bool   `json:"italic,omitempty"`
	Underline bool   `json:"underline,omitempty"`
}

// Theme defines all visual styling for rho.
type Theme struct {
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Dark        bool                  `json:"dark"`
	Author      string                `json:"author,omitempty"`
	Colors      map[string]ThemeColor `json:"colors"`
	Styles      map[string]string     `json:"styles"` // ANSI format strings
}

// BuiltinThemes returns the built-in themes.
func BuiltinThemes() []Theme {
	return []Theme{
		DefaultTheme(),
		DraculaTheme(),
		CatppuccinTheme(),
	}
}

// DefaultTheme returns the default light theme.
func DefaultTheme() Theme {
	return Theme{
		Name:  "default",
		Dark:  false,
		Colors: map[string]ThemeColor{
			"bg":          {Name: "bg", ANSI: 0, Hex: "#ffffff"},
			"fg":          {Name: "fg", ANSI: 7, Hex: "#000000"},
			"accent":      {Name: "accent", ANSI: 4, Hex: "#0066cc"},
			"success":     {Name: "success", ANSI: 2, Hex: "#00aa00"},
			"warning":     {Name: "warning", ANSI: 3, Hex: "#cc8800"},
			"error":       {Name: "error", ANSI: 1, Hex: "#cc0000"},
			"info":        {Name: "info", ANSI: 6, Hex: "#0099cc"},
			"subtle":      {Name: "subtle", ANSI: 8, Hex: "#888888"},
			"highlight":   {Name: "highlight", ANSI: 5, Hex: "#aa00aa"},
		},
		Styles: map[string]string{
			"title":       "\x1b[1;34m",   // Bold blue
			"user":        "\x1b[1;32m",   // Bold green
			"assistant":   "\x1b[1;34m",   // Bold blue
			"tool":        "\x1b[33m",     // Yellow
			"error":       "\x1b[1;31m",   // Bold red
			"code":        "\x1b[33m",     // Yellow
			"codeblock":   "\x1b[32m",     // Green
			"link":        "\x1b[34m",     // Blue
			"dim":         "\x1b[2m",      // Dim
			"bold":        "\x1b[1m",      // Bold
			"italic":      "\x1b[3m",      // Italic
			"separator":   "\x1b[90m",     // Bright black
		},
	}
}

// DraculaTheme returns a dark Dracula-inspired theme.
func DraculaTheme() Theme {
	return Theme{
		Name:  "dracula",
		Dark:  true,
		Colors: map[string]ThemeColor{
			"bg":          {Hex: "#282a36"},
			"fg":          {Hex: "#f8f8f2"},
			"accent":      {Hex: "#bd93f9"},
			"success":     {Hex: "#50fa7b"},
			"warning":     {Hex: "#f1fa8c"},
			"error":       {Hex: "#ff5555"},
			"info":        {Hex: "#8be9fd"},
			"subtle":      {Hex: "#6272a4"},
			"highlight":   {Hex: "#ff79c6"},
		},
		Styles: map[string]string{
			"title":       "\x1b[1;35m",
			"user":        "\x1b[1;32m",
			"assistant":   "\x1b[1;35m",
			"tool":        "\x1b[33m",
			"error":       "\x1b[1;31m",
			"code":        "\x1b[93m",
			"dim":         "\x1b[2m",
			"bold":        "\x1b[1m",
		},
	}
}

// CatppuccinTheme returns a Catppuccin Mocha-inspired dark theme.
func CatppuccinTheme() Theme {
	return Theme{
		Name:  "catppuccin",
		Dark:  true,
		Colors: map[string]ThemeColor{
			"bg":          {Hex: "#1e1e2e"},
			"fg":          {Hex: "#cdd6f4"},
			"accent":      {Hex: "#89b4fa"},
			"success":     {Hex: "#a6e3a1"},
			"warning":     {Hex: "#f9e2af"},
			"error":       {Hex: "#f38ba8"},
			"info":        {Hex: "#89dceb"},
			"subtle":      {Hex: "#585b70"},
			"highlight":   {Hex: "#cba6f7"},
		},
		Styles: map[string]string{
			"title":       "\x1b[38;5;117m",
			"user":        "\x1b[38;5;120m",
			"assistant":   "\x1b[38;5;117m",
			"tool":        "\x1b[38;5;228m",
			"error":       "\x1b[38;5;210m",
			"code":        "\x1b[38;5;228m",
			"dim":         "\x1b[2m",
			"bold":        "\x1b[1m",
		},
	}
}

// ThemeManager manages themes: loading, switching, and applying.
type ThemeManager struct {
	mu       sync.RWMutex
	themes   map[string]Theme
	active   string
	themeDir string
}

// NewThemeManager creates a new theme manager.
func NewThemeManager(themeDir string) *ThemeManager {
	tm := &ThemeManager{
		themes:   make(map[string]Theme),
		themeDir: themeDir,
	}

	// Load built-in themes
	for _, t := range BuiltinThemes() {
		tm.themes[t.Name] = t
	}

	tm.active = "default"
	return tm
}

// LoadThemes loads themes from the theme directory.
func (tm *ThemeManager) LoadThemes() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	entries, err := os.ReadDir(tm.themeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot read themes dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(tm.themeDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var t Theme
		if err := json.Unmarshal(data, &t); err != nil {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".json")
		if t.Name == "" {
			t.Name = name
		}
		tm.themes[t.Name] = t
	}

	return nil
}

// GetTheme returns a theme by name.
func (tm *ThemeManager) GetTheme(name string) (Theme, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	t, ok := tm.themes[name]
	return t, ok
}

// SetActive sets the active theme by name.
func (tm *ThemeManager) SetActive(name string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, ok := tm.themes[name]; !ok {
		return fmt.Errorf("theme not found: %s", name)
	}
	tm.active = name
	return nil
}

// Active returns the currently active theme.
func (tm *ThemeManager) Active() Theme {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.themes[tm.active]
}

// ActiveName returns the name of the active theme.
func (tm *ThemeManager) ActiveName() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.active
}

// ListThemes returns all available theme names.
func (tm *ThemeManager) ListThemes() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var names []string
	for name := range tm.themes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Style returns the ANSI escape sequence for a given style name.
func (tm *ThemeManager) Style(styleName string) string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	if t, ok := tm.themes[tm.active]; ok {
		if s, ok := t.Styles[styleName]; ok {
			return s
		}
	}
	return ""
}

// Reset returns the ANSI reset sequence.
func Reset() string {
	return "\x1b[0m"
}

// ApplyStyle wraps text with a style and reset.
func (tm *ThemeManager) ApplyStyle(styleName, text string) string {
	return tm.Style(styleName) + text + Reset()
}

// SaveTheme saves a theme to a JSON file.
func SaveTheme(t Theme, path string) error {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal theme: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create theme dir: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}
