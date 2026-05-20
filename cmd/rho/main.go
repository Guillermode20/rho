// rho - A lightweight extensible TUI coding agent (Go translation of pi)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/auth"
	"github.com/earendil-works/rho/pkg/agent/codecore"
	"github.com/earendil-works/rho/pkg/agent/rpc"
	agenttheme "github.com/earendil-works/rho/pkg/agent/theme"
	"github.com/earendil-works/rho/pkg/agent/tools"
	"github.com/earendil-works/rho/pkg/ai"
	"github.com/earendil-works/rho/pkg/ai/providers"
)

const version = "0.2.0"

func main() {
	printHelp := flag.Bool("help", false, "Show help")
	printVersion := flag.Bool("version", false, "Show version")
	mode := flag.String("mode", "interactive", "Run mode: interactive, print, json, rpc")
	modelName := flag.String("model", "", "Model to use (e.g., \"claude-sonnet-4-20250514\", \"gpt-4o\", \"gemini-2.5-pro-exp-03-25\")")
	providerName := flag.String("provider", "", "Provider to use (e.g., \"anthropic\", \"openai\", \"google\")")
	apiKey := flag.String("api-key", "", "API key for the provider")
	systemPrompt := flag.String("system-prompt", codecore.DefaultSystemPrompt, "System prompt for the agent")
	prompt := flag.String("prompt", "", "Single prompt (print mode)")
	cwd := flag.String("cwd", "", "Working directory (default: current directory)")
	listModels := flag.Bool("list-models", false, "List available models")

	flag.Parse()

	if *printHelp {
		printUsage()
		return
	}

	if *printVersion {
		fmt.Printf("rho v%s\n", version)
		return
	}

	// Resolve working directory
	workDir := *cwd
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot get working directory: %v\n", err)
			os.Exit(1)
		}
	}
	authStorage := auth.NewAuthStorage(defaultAuthKeysPath())
	oauthStore := auth.NewOAuthStore(defaultOAuthPath())
	rhoDir := filepath.Join(os.Getenv("HOME"), ".rho")
	settingsManager := codecore.NewSettingsManager(filepath.Join(rhoDir, "settings"), workDir)
	themeManager := agenttheme.NewThemeManager(filepath.Join(rhoDir, "themes"))
	if err := themeManager.LoadThemes(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load themes: %v\n", err)
	}
	if selectedTheme := settingsManager.GetString("theme"); selectedTheme != "" {
		if err := themeManager.SetActive(selectedTheme); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not activate theme %q: %v\n", selectedTheme, err)
		}
	}

	// Resolve model
	var model ai.Model
	if *modelName != "" {
		model = resolveModel(*modelName, *providerName)
	} else {
		model = getDefaultModel(authStorage, oauthStore)
	}

	// Resolve API key
	if *apiKey == "" {
		*apiKey = resolveAuthToken(model, authStorage, oauthStore)
	}

	// Resolve provider name
	if *providerName == "" {
		*providerName = string(model.Provider)
	}
	if model.Provider == "" {
		model.Provider = ai.Provider(*providerName)
	}

	// Check if we have a usable model with auth configured
	hasAuth := *apiKey != "" || (model.Name != "" && providerHasAuth(model.Provider, authStorage, oauthStore))
	if !hasAuth && model.Name != "" {
		// Model was auto-selected but no auth is configured - reset to let interactive mode handle it
		if *mode == "interactive" {
			model = ai.Model{}
		}
	}

	if *listModels {
		printModelList(authStorage)
		return
	}

	// Build runtime config — include extensions from ~/.rho/extensions/
	cfg := &RuntimeConfig{
		Model:        model,
		SystemPrompt: *systemPrompt,
		APIKey:       *apiKey,
		Provider:     model.Provider,
		CWD:          workDir,
		ExtDirs:      []string{codecore.GetExtensionsDir()},
		AuthStorage:  authStorage,
		OAuthStore:   oauthStore,
		Settings:     settingsManager,
		ThemeManager: themeManager,
	}

	switch *mode {
	case "interactive":
		im := NewInteractiveMode(cfg)
		if err := im.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "print":
		if err := runPrintMode(cfg, *prompt); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "json":
		if err := runJSONMode(cfg, *prompt); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "rpc":
		if err := runRPCMode(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", *mode)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`rho - A lightweight extensible TUI coding agent

Usage:
  rho [flags] [prompt]

Flags:
  -help            Show this help message
  -version         Show version
  -mode            Run mode: interactive (default), print, json, rpc
  -model           Model to use (e.g., "claude-sonnet-4-20250514", "gpt-4o")
  -provider        Provider to use (e.g., "anthropic", "openai", "google")
  -api-key         API key for the provider
  -system-prompt   System prompt for the agent
  -prompt          Single prompt (print mode)
  -cwd             Working directory (default: current directory)
  -list-models     List available models

Environment Variables:
  ANTHROPIC_API_KEY      API key for Anthropic
  OPENAI_API_KEY         API key for OpenAI
  GOOGLE_API_KEY         API key for Google Generative AI
  DEEPSEEK_API_KEY       API key for DeepSeek

Interactive Commands:
  /help                  List available slash commands
  /login [provider]      Save an API key for a provider
  /logout [provider]     Remove a saved API key

Examples:
  rho
  rho -model gpt-4o -provider openai -prompt "Hello"
  rho -mode print -prompt "List files" -cwd /path/to/project
  rho -list-models
`)
}

func resolveModel(modelName, providerName string) ai.Model {
	for _, def := range ai.DefaultModels() {
		if def.Name == modelName {
			if providerName == "" || string(def.Provider) == providerName {
				return ai.Model{
					API:      def.API,
					Provider: def.Provider,
					Name:     def.Name,
					BaseURL:  def.BaseURL,
				}
			}
		}
	}

	// If model has a provider prefix like "anthropic/claude-sonnet-4", parse it
	if strings.Contains(modelName, "/") {
		parts := strings.SplitN(modelName, "/", 2)
		providerName = parts[0]
		modelName = parts[1]
		for _, def := range ai.DefaultModels() {
			if def.Name == modelName && string(def.Provider) == providerName {
				return ai.Model{
					API:      def.API,
					Provider: def.Provider,
					Name:     def.Name,
					BaseURL:  def.BaseURL,
				}
			}
		}
	}

	// Fallback: construct model from parts
	api := ai.APIOpenAICompletions
	prov := ai.Provider(providerName)
	if prov == "" {
		prov = ai.ProviderOpenAI
	}

	// Auto-detect API from provider
	switch prov {
	case ai.ProviderAnthropic:
		api = ai.APIAnthropicMessages
	case ai.ProviderGoogle:
		api = ai.APIGoogleGenerativeAI
	case ai.ProviderDeepSeek:
		api = ai.APIOpenAICompletions
	case ai.ProviderCrof:
		api = ai.APIOpenAICompletions
	}

	return ai.Model{
		API:      api,
		Provider: prov,
		Name:     modelName,
	}
}

func getDefaultModel(authStorage *auth.AuthStorage, oauthStore *auth.OAuthStore) ai.Model {
	authCheck := func(provider ai.Provider) bool {
		return providerHasAuth(provider, authStorage, oauthStore)
	}
	available := ai.AvailableModels(authCheck)
	if len(available) > 0 {
		// Prefer Anthropic models first, then OpenAI, then first available
		preferred := []ai.Provider{ai.ProviderAnthropic, ai.ProviderOpenAI, ai.ProviderGoogle}
		for _, p := range preferred {
			for _, m := range available {
				if m.Provider == p {
					return ai.Model{
						API:      m.API,
						Provider: m.Provider,
						Name:     m.Name,
						BaseURL:  m.BaseURL,
					}
				}
			}
		}
		// Fallback to first available
		m := available[0]
		return ai.Model{
			API:      m.API,
			Provider: m.Provider,
			Name:     m.Name,
			BaseURL:  m.BaseURL,
		}
	}
	// No models with configured auth available - return empty model
	return ai.Model{}
}

// providerHasAuth checks if a provider has configured authentication via any source.
func providerHasAuth(provider ai.Provider, authStorage *auth.AuthStorage, oauthStore *auth.OAuthStore) bool {
	// Check stored API keys
	if authStorage != nil {
		if authStorage.HasAPIKey(string(provider)) {
			return true
		}
	}
	// Check OAuth credentials
	if oauthStore != nil {
		if oauthStore.HasProvider(string(provider)) {
			return true
		}
	}
	// Check environment variables
	if ai.ProviderHasEnvKey(provider) {
		return true
	}
	return false
}

func defaultAuthKeysPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".rho", "auth", "keys.json")
}

func defaultOAuthPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".rho", "auth", "oauth.json")
}

func resolveAPIKey(model ai.Model, authStorage *auth.AuthStorage) string {
	keys := ai.ProviderEnvKeys(model.Provider)
	if len(keys) > 0 {
		return firstConfiguredAPIKey(authStorage, string(model.Provider), keys...)
	}
	return providers.GetEnvAPIKey("OPENAI_API_KEY", "ANTHROPIC_API_KEY")
}

func resolveAuthToken(model ai.Model, authStorage *auth.AuthStorage, oauthStore *auth.OAuthStore) string {
	if key := resolveAPIKey(model, authStorage); strings.TrimSpace(key) != "" {
		return key
	}
	if oauthStore != nil {
		if cred, ok := oauthStore.Get(string(model.Provider)); ok && strings.TrimSpace(cred.AccessToken) != "" {
			// Check if OAuth credentials need refresh
			goCreds := &ai.OAuthCredentials{
				AccessToken:  cred.AccessToken,
				RefreshToken: cred.RefreshToken,
				ExpiresAt:    cred.ExpiresAt,
				ProviderID:   cred.Provider,
				Scopes:       cred.Scopes,
				TokenType:    cred.TokenType,
			}
			provider := ai.OAuthProviderFactory(ai.OAuthProviderID(cred.Provider))
			refreshed, err := ai.RefreshIfNeeded(goCreds, provider)
			if err == nil && refreshed != goCreds && strings.TrimSpace(refreshed.AccessToken) != "" {
				// Save refreshed credentials
				_ = oauthStore.Save(&auth.OAuthCredential{
					Provider:     refreshed.ProviderID,
					AccessToken:  refreshed.AccessToken,
					RefreshToken: refreshed.RefreshToken,
					ExpiresAt:    refreshed.ExpiresAt,
					Scopes:       refreshed.Scopes,
					TokenType:    refreshed.TokenType,
				})
				return refreshed.AccessToken
			}
			return cred.AccessToken
		}
	}
	return ""
}

func firstConfiguredAPIKey(authStorage *auth.AuthStorage, provider string, envNames ...string) string {
	if authStorage != nil {
		if key, ok := authStorage.GetAPIKey(provider); ok && strings.TrimSpace(key) != "" {
			return key
		}
	}
	return providers.GetEnvAPIKey(envNames...)
}

func resolveAPIKeyForProvider(provider ai.Provider, authStorage *auth.AuthStorage) string {
	// 1. Try env keys
	for _, envKey := range ai.ProviderEnvKeys(provider) {
		if val := os.Getenv(envKey); val != "" {
			return val
		}
	}
	// 2. Try authStorage
	if authStorage != nil {
		if val, ok := authStorage.GetAPIKey(string(provider)); ok && strings.TrimSpace(val) != "" {
			return val
		}
	}
	return ""
}

func printModelList(authStorage *auth.AuthStorage) {
	// Fetch models for all providers that have configured auth, in parallel
	uniqueProviders := make(map[ai.Provider]bool)
	for _, def := range ai.DefaultModels() {
		uniqueProviders[def.Provider] = true
	}

	var wg sync.WaitGroup
	for provider := range uniqueProviders {
		key := resolveAPIKeyForProvider(provider, authStorage)
		if key == "" {
			continue
		}
		wg.Add(1)
		provider := provider // capture
		go func() {
			defer wg.Done()
			// FetchModelsForProvider has its own 10s internal timeout
			defs, err := providers.FetchModelsForProvider(provider, key)
			if err == nil && len(defs) > 0 {
				ai.UpdateActiveProviderModels(provider, defs)
			}
		}()
	}
	wg.Wait()

	fmt.Println("Available models:")
	fmt.Println()
	for _, def := range ai.DefaultModels() {
		providerName := def.Provider
		reasoning := ""
		if def.Reasoning {
			reasoning = " [reasoning]"
		}
		// Check if this provider has any auth configured
		available := resolveAPIKeyForProvider(def.Provider, authStorage) != ""
		indicator := " "
		if !available {
			indicator = "⚠"
		}
		fmt.Printf("  %s %s/%s  (%s)%s\n", indicator, providerName, def.Name, string(def.API), reasoning)
	}
	fmt.Println()
	fmt.Println("  ⚠ = no API key found in environment variables or auth file")
	fmt.Println("  Use /login <provider> to configure an API key, or set the appropriate env var.")
}

func readPrompt(promptText string) (string, error) {
	if promptText == "" {
		args := flag.Args()
		if len(args) > 0 {
			promptText = strings.Join(args, " ")
		} else {
			// Read from stdin if piped
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return "", err
				}
				promptText = strings.TrimSpace(string(data))
			}
		}
		if promptText == "" {
			return "", fmt.Errorf("no prompt provided; use -prompt, pass text as arguments, or pipe input")
		}
	}

	return promptText, nil
}

func runPrintMode(cfg *RuntimeConfig, promptText string) error {
	promptText, err := readPrompt(promptText)
	if err != nil {
		return err
	}

	results, streamed, err := runAgentOnce(cfg, []agent.AgentMessage{{
		Role:    ai.RoleUser,
		Content: promptText,
	}}, true)
	if err != nil {
		return err
	}
	if streamed {
		fmt.Println()
		return nil
	}
	for i := len(results) - 1; i >= 0; i-- {
		if results[i].Role == ai.RoleAssistant && results[i].Content != "" {
			fmt.Println(results[i].Content)
			return nil
		}
	}
	return fmt.Errorf("no response generated")
}

func runJSONMode(cfg *RuntimeConfig, promptText string) error {
	var input struct {
		Messages []agent.AgentMessage `json:"messages"`
		Prompt   string               `json:"prompt"`
	}

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		if len(strings.TrimSpace(string(data))) > 0 {
			if err := json.Unmarshal(data, &input); err != nil {
				return fmt.Errorf("invalid json input: %w", err)
			}
		}
	}

	if len(input.Messages) == 0 {
		prompt := input.Prompt
		if prompt == "" {
			prompt = promptText
		}
		var err error
		prompt, err = readPrompt(prompt)
		if err != nil {
			return err
		}
		input.Messages = []agent.AgentMessage{{Role: ai.RoleUser, Content: prompt}}
	}

	results, _, err := runAgentOnce(cfg, input.Messages, false)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
		"messages": results,
	})
}

func runRPCMode(cfg *RuntimeConfig) error {
	rhoDir := filepath.Join(os.Getenv("HOME"), ".rho")
	server := rpc.NewServer()
	server.SetModel(cfg.Model)
	server.SetAPIKey(cfg.APIKey)
	server.SetTools(tools.AllTools(cfg.CWD))
	server.SetSessionManager(agent.NewSessionManager(filepath.Join(rhoDir, "sessions")))
	return server.Run()
}

func runAgentOnce(cfg *RuntimeConfig, messages []agent.AgentMessage, streamToStdout bool) ([]agent.AgentMessage, bool, error) {
	if len(messages) == 0 {
		return nil, false, fmt.Errorf("no messages provided")
	}

	prompt := messages[len(messages)-1]
	if prompt.Role != ai.RoleUser {
		return nil, false, fmt.Errorf("last message must be a user message")
	}

	context := agent.AgentContext{
		SystemPrompt: cfg.SystemPrompt,
		Model:        cfg.Model,
		Messages:     append([]agent.AgentMessage(nil), messages[:len(messages)-1]...),
		Tools:        tools.AllTools(cfg.CWD),
	}
	loop := agent.NewAgentLoop(agent.AgentLoopConfig{
		Model:             cfg.Model,
		SystemPrompt:      cfg.SystemPrompt,
		APIKey:            cfg.APIKey,
		ToolExecutionMode: agent.ToolExecutionSequential,
	})

	streamed := false
	results, err := loop.Run([]agent.AgentMessage{prompt}, context, func(event agent.AgentEvent) error {
		switch event.Type {
		case "text_delta":
			if streamToStdout {
				streamed = true
				fmt.Print(event.Delta)
			}
		case "tool_execution_start":
			if event.ToolCall != nil {
				fmt.Fprintf(os.Stderr, "Running tool: %s\n", event.ToolCall.Name)
			}
		}
		return nil
	})
	return results, streamed, err
}

type config struct {
	APIKey       string `json:"apiKey"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	SystemPrompt string `json:"systemPrompt"`
	Theme        string `json:"theme"`
}

func loadConfig(path string) (*config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &config{}, nil
		}
		return nil, err
	}
	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveConfig(path string, cfg *config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
