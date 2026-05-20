package extensions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/skills"
	"github.com/earendil-works/rho/pkg/ai"
)

// LoadExtensionsResult summarizes a load operation.
type LoadExtensionsResult struct {
	Loaded  []string `json:"loaded"`
	Errors  []string `json:"errors"`
	Skipped []string `json:"skipped"`
}

// LoadExtensions discovers and loads extensions from multiple directories.
func LoadExtensions(dirs []string, runtime *Runtime) *LoadExtensionsResult {
	result := &LoadExtensionsResult{}
	seen := make(map[string]bool)

	// Create dummy/default UI for initial load
	dummyUI := ExtensionUIContext{
		Notify: func(msg, t string) {
			fmt.Fprintf(os.Stderr, "Extension: %s\n", msg)
		},
	}

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
			if strings.HasPrefix(name, ".") {
				continue
			}

			if seen[name] {
				continue
			}
			seen[name] = true

			extDir := filepath.Join(dir, name)
			if _, err := os.Stat(filepath.Join(extDir, ".disabled")); err == nil {
				result.Skipped = append(result.Skipped, name)
				continue
			}

			if err := LoadExtensionFromDir(extDir, runtime, dummyUI); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("extension %s: %v", name, err))
				continue
			}

			result.Loaded = append(result.Loaded, name)
		}


	}

	sort.Strings(result.Loaded)
	return result
}

// LoadExtensionFromDir loads an extension from a directory.
func LoadExtensionFromDir(dir string, runtime *Runtime, ui ExtensionUIContext) error {
	// 1. Try to load from rho.toml manifest
	tomlPath := filepath.Join(dir, "rho.toml")
	if _, err := os.Stat(tomlPath); err == nil {
		data, err := os.ReadFile(tomlPath)
		if err != nil {
			return fmt.Errorf("cannot read rho.toml: %w", err)
		}

		manifest, err := ParseTOML(string(data))
		if err != nil {
			return fmt.Errorf("cannot parse rho.toml: %w", err)
		}

		ext := &ExtensionDef{
			Name:        manifest.Name,
			Description: manifest.Description,
			Version:     manifest.Version,
		}

		var initRes struct {
			Tools []struct {
				ID          string      `json:"id"`
				Description string      `json:"description"`
				Parameters  interface{} `json:"parameters"`
			} `json:"tools"`
			Commands []struct {
				Name        string   `json:"name"`
				Description string   `json:"description"`
				Args        []string `json:"args"`
			} `json:"commands"`
		}

		// Spawns and registers extension process if entry command exists
		if manifest.Entry.Command != "" {
			proc := NewExtensionProcess(dir, manifest)
			runtime.RegisterProcess(manifest.Name, proc)

			if err := proc.Start(ui); err != nil {
				return fmt.Errorf("cannot start process: %w", err)
			}

			initMsg, err := proc.Call("initialize", map[string]string{"rhoVersion": "0.1.0"})
			if err == nil {
				_ = json.Unmarshal(initMsg.Result, &initRes)
			}

			// Map tools
			for _, t := range initRes.Tools {
				toolID := t.ID
				ext.CustomTools = append(ext.CustomTools, ToolDefinition{
					Name:        toolID,
					Label:       toolID,
					Description: t.Description,
					Parameters:  t.Parameters,
					Execute: func(args map[string]interface{}) (string, bool, error) {
						resp, err := proc.Call("tool.call", map[string]interface{}{
							"tool":  toolID,
							"input": args,
						})
						if err != nil {
							return "", true, err
						}
						if resp == nil || resp.Result == nil {
							return "", false, nil
						}
						var toolResult struct {
							Content string `json:"content"`
							IsError bool   `json:"isError"`
						}
						if err := json.Unmarshal(resp.Result, &toolResult); err != nil {
							return "", true, err
						}
						return toolResult.Content, toolResult.IsError, nil
					},
				})
			}

			// Map commands
			for _, cmd := range initRes.Commands {
				cmdName := cmd.Name
				ext.SlashCommands = append(ext.SlashCommands, SlashCommand{
					Name:        cmdName,
					Description: cmd.Description,
					Args:        cmd.Args,
					Handler: func(ctx ExtensionContext, args []string) error {
						proc.mu.Lock()
						proc.ui = ctx.UI
						proc.mu.Unlock()
						_, err := proc.Call("command.call", map[string]interface{}{
							"command": cmdName,
							"args":    args,
						})
						return err
					},
				})
			}

			// Wire lifecycle events
			ext.OnSessionStart = func(ctx ExtensionContext, event SessionStartEvent) error {
				proc.mu.Lock()
				proc.ui = ctx.UI
				proc.mu.Unlock()
				if err := proc.Start(ctx.UI); err != nil {
					return err
				}
				_, err := proc.Call("lifecycle.event", map[string]interface{}{
					"event": "session_start",
					"data":  event,
				})
				return err
			}

			ext.OnSessionShutdown = func(ctx ExtensionContext, event SessionShutdownEvent) error {
				_, _ = proc.Call("lifecycle.event", map[string]interface{}{
					"event": "session_shutdown",
					"data":  event,
				})
				proc.Stop()
				return nil
			}

			ext.OnAgentStart = func(ctx ExtensionContext) error {
				proc.mu.Lock()
				proc.ui = ctx.UI
				proc.mu.Unlock()
				_, err := proc.Call("lifecycle.event", map[string]interface{}{
					"event": "agent_start",
				})
				return err
			}

			ext.OnAgentEnd = func(ctx ExtensionContext, event AgentEndEvent) error {
				_, err := proc.Call("lifecycle.event", map[string]interface{}{
					"event": "agent_end",
					"data":  event,
				})
				return err
			}

			ext.OnTurnStart = func(ctx ExtensionContext, event TurnStartEvent) error {
				proc.mu.Lock()
				proc.ui = ctx.UI
				proc.mu.Unlock()
				_, err := proc.Call("lifecycle.event", map[string]interface{}{
					"event": "turn_start",
					"data":  event,
				})
				return err
			}

			ext.OnTurnEnd = func(ctx ExtensionContext, event TurnEndEvent) error {
				_, err := proc.Call("lifecycle.event", map[string]interface{}{
					"event": "turn_end",
					"data":  event,
				})
				return err
			}

			ext.OnContext = func(ctx ExtensionContext, event ContextEvent) ([]agent.AgentMessage, error) {
				resp, err := proc.Call("lifecycle.event", map[string]interface{}{
					"event": "context",
					"data":  event,
				})
				if err != nil {
					return nil, err
				}
				if resp == nil || resp.Result == nil || isJSONString(resp.Result) {
					return nil, nil
				}
				var res []agent.AgentMessage
				if err := json.Unmarshal(resp.Result, &res); err != nil {
					return nil, err
				}
				return res, nil
			}

			ext.OnBeforeProviderRequest = func(ctx ExtensionContext, event BeforeProviderRequestEvent) (interface{}, error) {
				resp, err := proc.Call("lifecycle.event", map[string]interface{}{
					"event": "before_provider_request",
					"data":  event,
				})
				if err != nil {
					return nil, err
				}
				if resp == nil || resp.Result == nil || isJSONString(resp.Result) {
					return nil, nil
				}
				var res interface{}
				if err := json.Unmarshal(resp.Result, &res); err != nil {
					return nil, err
				}
				return res, nil
			}

			ext.OnBeforeAgentStart = func(ctx ExtensionContext, event BeforeAgentStartEvent) error {
				proc.mu.Lock()
				proc.ui = ctx.UI
				proc.mu.Unlock()
				_, err := proc.Call("lifecycle.event", map[string]interface{}{
					"event": "before_agent_start",
					"data":  event,
				})
				return err
			}

			ext.OnInput = func(ctx ExtensionContext, event InputEvent) (*InputEventResult, error) {
				proc.mu.Lock()
				proc.ui = ctx.UI
				proc.mu.Unlock()
				resp, err := proc.Call("lifecycle.event", map[string]interface{}{
					"event": "input",
					"data":  event,
				})
				if err != nil {
					return nil, err
				}
				if resp == nil || resp.Result == nil || isJSONString(resp.Result) {
					return nil, nil
				}
				var res InputEventResult
				if err := json.Unmarshal(resp.Result, &res); err != nil {
					return nil, err
				}
				return &res, nil
			}

			ext.OnToolCall = func(ctx ExtensionContext, event ToolCallEvent) (*ToolCallEventResult, error) {
				proc.mu.Lock()
				proc.ui = ctx.UI
				proc.mu.Unlock()
				resp, err := proc.Call("lifecycle.event", map[string]interface{}{
					"event": "tool_call",
					"data":  event,
				})
				if err != nil {
					return nil, err
				}
				if resp == nil || resp.Result == nil || isJSONString(resp.Result) {
					return nil, nil
				}
				var res ToolCallEventResult
				if err := json.Unmarshal(resp.Result, &res); err != nil {
					return nil, err
				}
				return &res, nil
			}

			ext.OnToolResult = func(ctx ExtensionContext, event ToolResultEvent) error {
				_, err := proc.Call("lifecycle.event", map[string]interface{}{
					"event": "tool_result",
					"data":  event,
				})
				return err
			}

			ext.OnUserBash = func(ctx ExtensionContext, event UserBashEvent) error {
				_, err := proc.Call("lifecycle.event", map[string]interface{}{
					"event": "user_bash",
					"data":  event,
				})
				return err
			}
		}

		// Load skills
		if manifest.Capabilities.Skills {
			skillsDir := filepath.Join(dir, "skills")
			if info, err := os.Stat(skillsDir); err == nil && info.IsDir() {
				res := skills.LoadSkillsFromDir(skillsDir)
				runtime.RegisterSkills(manifest.Name, res.Loaded)
			}
		}

		// Load prompts
		if manifest.Capabilities.Prompts {
			promptsDir := filepath.Join(dir, "prompts")
			if info, err := os.Stat(promptsDir); err == nil && info.IsDir() {
				prompts := loadPromptsFromDir(promptsDir)
				runtime.RegisterPromptPatches(manifest.Name, prompts)
			}
		}

		runtime.Register(ext)

		// Set up file watcher for hot reloading
		runtime.WatchExtensionDir(manifest.Name, dir, ui)

		return nil
	}

	// 2. Fallback to extension.json config
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
			tool := t
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

// isJSONString returns true when the raw JSON message is a string value (starts with ").
// Used to distinguish pass-through "ok" responses from structured data in lifecycle events.
func isJSONString(raw json.RawMessage) bool {
	return len(raw) > 0 && raw[0] == '"'
}

func loadPromptsFromDir(dir string) []string {
	var prompts []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".txt" || ext == ".md" {
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to read prompt file %s: %v\n", filepath.Join(dir, entry.Name()), err)
				continue
			}
			prompts = append(prompts, string(data))
		}
	}
	return prompts
}
