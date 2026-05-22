package codecore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type SettingsError struct {
	Scope string `json:"scope"`
	Error error  `json:"error"`
}

type SettingsManager struct {
	mu           sync.RWMutex
	settings     map[string]interface{}
	projectPath  string
	userPath     string
	defaultsPath string
	errors       []SettingsError
}

func NewSettingsManager(userDir, projectDir string) *SettingsManager {
	sm := &SettingsManager{
		settings:     make(map[string]interface{}),
		userPath:     filepath.Join(userDir, "settings.json"),
		defaultsPath: filepath.Join(userDir, "defaults.json"),
	}
	if projectDir != "" {
		sm.projectPath = filepath.Join(projectDir, ".rho", "settings.json")
	}
	sm.load()
	return sm
}

func (sm *SettingsManager) Get(key string) interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.projectPath != "" {
		if v := sm.getFromFile(sm.projectPath, key); v != nil {
			return v
		}
	}
	if v := sm.getFromFile(sm.userPath, key); v != nil {
		return v
	}
	if v := sm.getFromFile(sm.defaultsPath, key); v != nil {
		return v
	}
	return sm.settings[key]
}

func (sm *SettingsManager) GetString(key string) string {
	if s, ok := sm.Get(key).(string); ok {
		return s
	}
	return ""
}

func (sm *SettingsManager) GetBool(key string) bool {
	if b, ok := sm.Get(key).(bool); ok {
		return b
	}
	return false
}

func (sm *SettingsManager) GetInt(key string) int {
	switch v := sm.Get(key).(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

func (sm *SettingsManager) SetUser(key string, value interface{}) error {
	return sm.setInFile(sm.userPath, key, value)
}

func (sm *SettingsManager) SetProject(key string, value interface{}) error {
	if sm.projectPath == "" {
		return fmt.Errorf("no project path")
	}
	return sm.setInFile(sm.projectPath, key, value)
}

func (sm *SettingsManager) GetAll() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	r := make(map[string]interface{})
	for k, v := range sm.settings {
		r[k] = v
	}
	sm.mergeFromFile(sm.defaultsPath, r)
	sm.mergeFromFile(sm.userPath, r)
	if sm.projectPath != "" {
		sm.mergeFromFile(sm.projectPath, r)
	}
	return r
}

func (sm *SettingsManager) DrainErrors() []SettingsError {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	e := sm.errors
	sm.errors = nil
	return e
}

func (sm *SettingsManager) load() {
	for _, p := range []string{sm.defaultsPath, sm.userPath, sm.projectPath} {
		if p == "" {
			continue
		}
		if err := sm.loadFile(p); err != nil && !os.IsNotExist(err) {
			scope := "project"
			if p == sm.defaultsPath {
				scope = "defaults"
			} else if p == sm.userPath {
				scope = "user"
			}
			sm.errors = append(sm.errors, SettingsError{Scope: scope, Error: err})
		}
	}
}

func (sm *SettingsManager) loadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var vals map[string]interface{}
	if err := json.Unmarshal(data, &vals); err != nil {
		return err
	}
	for k, v := range vals {
		sm.settings[k] = v
	}
	return nil
}

func (sm *SettingsManager) getFromFile(path, key string) interface{} {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var vals map[string]interface{}
	if json.Unmarshal(data, &vals) != nil {
		return nil
	}
	return vals[key]
}

func (sm *SettingsManager) setInFile(path, key string, value interface{}) error {
	os.MkdirAll(filepath.Dir(path), 0755)
	var vals map[string]interface{}
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &vals)
	}
	if vals == nil {
		vals = make(map[string]interface{})
	}
	vals[key] = value
	d, err := json.MarshalIndent(vals, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, d, 0644)
}

func (sm *SettingsManager) mergeFromFile(path string, target map[string]interface{}) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var vals map[string]interface{}
	if json.Unmarshal(data, &vals) != nil {
		return
	}
	for k, v := range vals {
		target[k] = v
	}
}
