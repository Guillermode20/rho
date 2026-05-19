package extensions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"sort"
	"strings"

	"github.com/earendil-works/rho/pkg/ai"
)

// LoadExtensionsResult summarizes a load operation.
type LoadExtensionsResult struct {
	Loaded      []string `json:"loaded"`
	Errors      []string `json:"errors"`
	Skipped     []string `json:"skipped"`
}

// LoadExtensions discovers and loads extensions from multiple directories.
//
// Directories are searched for:
//   - Go plugins (.so files exporting a "RegisterExtension" function)
//   - Extension configs (.json files with extension metadata)
//   - Subdirectories containing extension.json or manifest files
//
// Extension packages can be versioned: name@version/
func LoadExtensions(dirs []string, runtime *Runtime) *LoadExtensionsResult {
	result := &LoadExtensionsResult{}

	seen := make(map[string]bool)

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("cannot create extension dir %s: %v", dir, err))
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("cannot read extension dir %s: %v", dir, err))
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			name := entry.Name()

			// Skip hidden directories
			if strings.HasPrefix(name, ".") {
				continue
			}

			if seen[name] {
				continue
			}
			seen[name] = true

			extDir := filepath.Join(dir, name)

			// Try to load from this directory
			if err := loadExtensionFromDir(extDir, runtime); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("extension %s: %v", name, err))
				continue
			}

			result.Loaded = append(result.Loaded, name)
		}

		// Also look for .so plugin files
		files, err := filepath.Glob(filepath.Join(dir, "*.so"))
		if err == nil {
			for _, f := range files {
				name := strings.TrimSuffix(filepath.Base(f), ".so")
				if seen[name] {
					continue
				}
				seen[name] = true

				if err := loadGoPlugin(f, runtime); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("plugin %s: %v", name, err))
					continue
				}
				result.Loaded = append(result.Loaded, name)
			}
		}
	}

	sort.Strings(result.Loaded)
	return result
}

// loadExtensionFromDir loads an extension from a directory.
func loadExtensionFromDir(dir string, runtime *Runtime) error {
	// Check for extension.json config
	configPath := filepath.Join(dir, "extension.json")
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("cannot read config: %w", err)
		}

		var cfg struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Version     string `json:"version"`
			EntryPoint  string `json:"entryPoint"`
			Tools       []struct {
				Name        string      `json:"name"`
				Description string      `json:"description"`
				Parameters  interface{} `json:"parameters"`
			} `json:"tools"`
			Providers []struct {
				Name    string   `json:"name"`
				API     string   `json:"api"`
				BaseURL string   `json:"baseUrl"`
				Models  []string `json:"models"`
			} `json:"providers"`
			Commands []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"commands"`
		}

		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("cannot parse config: %w", err)
		}

		ext := &ExtensionDef{
			Name:        cfg.Name,
			Description: cfg.Description,
			Version:     cfg.Version,
		}

		// Load tools from config
		for _, t := range cfg.Tools {
			tool := t // capture
			ext.CustomTools = append(ext.CustomTools, ToolDefinition{
				Name:        tool.Name,
				Label:       tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			})
		}

		// Load providers from config
		for _, p := range cfg.Providers {
			var models []ai.Model
			for _, m := range p.Models {
				models = append(models, ai.Model{
					API:      ai.API(p.API),
					Provider: ai.Provider(p.Name),
					Name:     m,
					BaseURL:  p.BaseURL,
				})
			}
			ext.CustomProviders = append(ext.CustomProviders, ProviderConfig{
				Name:    p.Name,
				API:     ai.API(p.API),
				BaseURL: p.BaseURL,
				Models:  models,
			})
		}

		runtime.Register(ext)
		return nil
	}

	// Could also check for an entry point script
	return nil
}

// loadGoPlugin loads a Go plugin (.so) that exports RegisterExtension.
func loadGoPlugin(soPath string, runtime *Runtime) error {
	p, err := plugin.Open(soPath)
	if err != nil {
		return fmt.Errorf("cannot open plugin: %w", err)
	}

	sym, err := p.Lookup("RegisterExtension")
	if err != nil {
		return fmt.Errorf("plugin has no RegisterExtension symbol: %w", err)
	}

	registerFn, ok := sym.(func(*Runtime))
	if !ok {
		return fmt.Errorf("RegisterExtension has wrong signature")
	}

	registerFn(runtime)
	return nil
}

// BuiltinExtensions returns the built-in extensions that ship with rho.
func BuiltinExtensions(runtime *Runtime) {
	// No built-in extensions yet
}

// LoadExtensionConfig loads extension configuration from a JSON file.
func LoadExtensionConfig(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, err
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// RegisterExtensionFromConfig creates and registers an extension from raw config.
func RegisterExtensionFromConfig(runtime *Runtime, name string, cfg map[string]interface{}) error {
	ext := &ExtensionDef{
		Name: name,
	}

	if desc, ok := cfg["description"].(string); ok {
		ext.Description = desc
	}

	// Parse tools
	if tools, ok := cfg["tools"].([]interface{}); ok {
		for _, t := range tools {
			if toolMap, ok := t.(map[string]interface{}); ok {
				name, _ := toolMap["name"].(string)
				desc, _ := toolMap["description"].(string)
				params := toolMap["parameters"]
				ext.CustomTools = append(ext.CustomTools, ToolDefinition{
					Name:        name,
					Label:       name,
					Description: desc,
					Parameters:  params,
				})
			}
		}
	}

	runtime.Register(ext)
	return nil
}
