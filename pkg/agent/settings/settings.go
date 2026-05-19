// Package settings manages user configuration with scoped overrides.
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// SettingsScope represents the scope of a setting.
type SettingsScope string

const (
	ScopeDefault SettingsScope = "default"
	ScopeUser    SettingsScope = "user"
	ScopeProject SettingsScope = "project"
)

// SettingValue holds a typed setting value.
type SettingValue struct {
	Value   interface{}   `json:"value"`
	Scope   SettingsScope `json:"scope"`
	Source  string        `json:"source,omitempty"`
}

// SettingsManager manages configuration with scoped overrides.
type SettingsManager struct {
	mu          sync.RWMutex
	configDir   string
	projectDirs []string
	scoped      map[string]SettingsScope
	values      map[string]interface{}
	defaults    map[string]interface{}
	errors      []SettingsError
}

// SettingsError captures a configuration error.
type SettingsError struct {
	Scope   string `json:"scope"`
	Message string `json:"message"`
}

// NewSettingsManager creates a new settings manager with default values.
func NewSettingsManager(configDir string) *SettingsManager {
	sm := &SettingsManager{
		configDir:   configDir,
		values:      make(map[string]interface{}),
		defaults:    make(map[string]interface{}),
		scoped:      make(map[string]SettingsScope),
	}

	// Set defaults
	sm.SetDefaults()

	return sm
}

// SetDefaults establishes the default configuration values.
func (sm *SettingsManager) SetDefaults() {
	sm.defaults = map[string]interface{}{
		// Model settings
		"model":               "",
		"provider":            "",
		"maxTokens":           8192,
		"temperature":         0.7,
		"thinkingLevel":       "off",

		// UI settings
		"theme":               "default",
		"showImages":          true,
		"hardwareCursor":      false,
		"clearOnShrink":       true,
		"showLineNumbers":     true,

		// Tool settings
		"autoResizeImages":    true,
		"maxReadBytes":        50000,
		"maxReadLines":        2000,
		"maxBashOutput":       100000,
		"bashTimeout":         0,

		// Session settings
		"autoSave":            true,
		"autoCompact":         true,
		"compactTargetTokens": 40000,
		"compactMaxTokens":    80000,
		"sessionMaxAge":       "30d",

		// Extension settings
		"extensionsDir":       "",
		"autoLoadExtensions":  true,

		// Feature flags
		"kittyProtocol":       true,
		"modifyOtherKeys":     true,
	}

	// Copy defaults to values
	for k, v := range sm.defaults {
		sm.values[k] = v
	}
}

// AddProjectDir adds a project directory for project-scoped settings.
func (sm *SettingsManager) AddProjectDir(dir string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.projectDirs = append(sm.projectDirs, dir)
}

// Load loads settings from all scopes.
func (sm *SettingsManager) Load() []SettingsError {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.errors = nil

	// Reset to defaults
	for k, v := range sm.defaults {
		sm.values[k] = v
		sm.scoped[k] = ScopeDefault
	}

	// Load user config
	userPath := filepath.Join(sm.configDir, "config.json")
	if err := sm.loadFile(userPath, ScopeUser); err != nil {
		sm.errors = append(sm.errors, SettingsError{
			Scope:   "user",
			Message: err.Error(),
		})
	}

	// Load project config
	for _, dir := range sm.projectDirs {
		projectPath := filepath.Join(dir, ".rho", "config.json")
		if _, err := os.Stat(projectPath); err == nil {
			if err := sm.loadFile(projectPath, ScopeProject); err != nil {
				sm.errors = append(sm.errors, SettingsError{
					Scope:   "project",
					Message: err.Error(),
				})
			}
		}
	}

	// Apply environment variable overrides
	sm.loadEnvironment()

	return sm.errors
}

// loadFile loads settings from a JSON file.
func (sm *SettingsManager) loadFile(path string, scope SettingsScope) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot read %s: %w", path, err)
	}

	var fileSettings map[string]interface{}
	if err := json.Unmarshal(data, &fileSettings); err != nil {
		return fmt.Errorf("cannot parse %s: %w", path, err)
	}

	for k, v := range fileSettings {
		sm.values[k] = v
		sm.scoped[k] = scope
	}

	return nil
}

// loadEnvironment loads settings from environment variables.
func (sm *SettingsManager) loadEnvironment() {
	envMappings := map[string]string{
		"RHO_MODEL":                "model",
		"RHO_PROVIDER":             "provider",
		"RHO_API_KEY":              "apiKey",
		"RHO_TEMPERATURE":          "temperature",
		"RHO_MAX_TOKENS":           "maxTokens",
		"RHO_THEME":                "theme",
		"RHO_EXTENSIONS_DIR":       "extensionsDir",
		"RHO_BASH_TIMEOUT":         "bashTimeout",
	}

	for envKey, settingKey := range envMappings {
		if val := os.Getenv(envKey); val != "" {
			sm.values[settingKey] = val
			sm.scoped[settingKey] = ScopeUser
		}
	}
}

// Get returns a setting value by key.
func (sm *SettingsManager) Get(key string) (interface{}, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	val, ok := sm.values[key]
	return val, ok
}

// GetString returns a string setting value.
func (sm *SettingsManager) GetString(key string) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if val, ok := sm.values[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// GetInt returns an int setting value.
func (sm *SettingsManager) GetInt(key string) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if val, ok := sm.values[key]; ok {
		switch v := val.(type) {
		case float64:
			return int(v)
		case int:
			return v
		}
	}
	return 0
}

// GetBool returns a bool setting value.
func (sm *SettingsManager) GetBool(key string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if val, ok := sm.values[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// Set sets a setting value at the given scope.
func (sm *SettingsManager) Set(key string, value interface{}, scope SettingsScope) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.values[key] = value
	sm.scoped[key] = scope

	// Persist to file for user/project scopes
	switch scope {
	case ScopeUser:
		return sm.persist(filepath.Join(sm.configDir, "config.json"), scope)
	case ScopeProject:
		for _, dir := range sm.projectDirs {
			path := filepath.Join(dir, ".rho", "config.json")
			return sm.persist(path, scope)
		}
	}

	return nil
}

// persist writes the current settings of a given scope to a file.
func (sm *SettingsManager) persist(path string, scope SettingsScope) error {
	scopeValues := make(map[string]interface{})
	for k, s := range sm.scoped {
		if s == scope {
			scopeValues[k] = sm.values[k]
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create config dir: %w", err)
	}

	data, err := json.MarshalIndent(scopeValues, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("cannot write config: %w", err)
	}

	return nil
}

// GetScope returns the scope of a setting.
func (sm *SettingsManager) GetScope(key string) SettingsScope {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if scope, ok := sm.scoped[key]; ok {
		return scope
	}
	return ScopeDefault
}

// All returns all settings with their scopes.
func (sm *SettingsManager) All() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make(map[string]interface{})
	for k, v := range sm.values {
		result[k] = v
	}
	return result
}

// DrainErrors returns and clears accumulated load errors.
func (sm *SettingsManager) DrainErrors() []SettingsError {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	errs := sm.errors
	sm.errors = nil
	return errs
}

// ShortName shortens a config key for display.
func ShortName(key string) string {
	parts := strings.Split(key, ".")
	return parts[len(parts)-1]
}
