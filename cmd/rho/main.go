// rho - A lightweight extensible TUI coding agent (Go translation of pi)
package main

import (
	"encoding/json"
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
	args := ParseArgs(os.Args[1:])

	// ── Package manager subcommands ────────────────────────────────────────────
	// Route install / uninstall / remove / update / list / info early (before
	// any other processing), matching pi's handlePackageCommand() behaviour.
	if subcmd := detectPackageSubcommand(os.Args[1:]); subcmd != "" {
		rhoDir := defaultRhoDir()
		pm := codecore.NewPackageManagerCLI(rhoDir)
		// Pass the raw args after the first positional (the subcommand).
		if err := pm.HandleCommand(os.Args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// ── config subcommand ──────────────────────────────────────────────────────
	if len(os.Args) > 1 && os.Args[1] == "config" {
		runConfigMode()
		return
	}

	// ── --help / -h ────────────────────────────────────────────────────────────
	if args.Help {
		printUsage()
		return
	}

	// ── --version / -v ────────────────────────────────────────────────────────
	if args.Version {
		fmt.Printf("rho v%s\n", version)
		return
	}

	// ── Resolve working directory ──────────────────────────────────────────────
	workDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot get working directory: %v\n", err)
		os.Exit(1)
	}

	authStorage := auth.NewAuthStorage(defaultAuthKeysPath())
	oauthStore := auth.NewOAuthStore(defaultOAuthPath())
	rhoDir := defaultRhoDir()
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

	// ── Resolve model ──────────────────────────────────────────────────────────
	var model ai.Model
	if args.Model != "" {
		model = resolveModel(args.Model, args.Provider)
	} else {
		model = getDefaultModel(authStorage, oauthStore)
	}

	// ── Resolve API key ────────────────────────────────────────────────────────
	apiKey := args.APIKey
	if apiKey == "" {
		apiKey = resolveAuthToken(model, authStorage, oauthStore)
	}

	// ── Resolve provider ───────────────────────────────────────────────────────
	providerName := args.Provider
	if providerName == "" {
		providerName = string(model.Provider)
	}
	if model.Provider == "" {
		model.Provider = ai.Provider(providerName)
	}

	// ── Auth guard ─────────────────────────────────────────────────────────────
	hasAuth := apiKey != "" || (model.Name != "" && providerHasAuth(model.Provider, authStorage, oauthStore))
	if !hasAuth && model.Name != "" {
		// Model was auto-selected but no auth is configured.
		resolvedMode := resolveMode(args)
		if resolvedMode == "interactive" {
			model = ai.Model{}
		}
	}

	// ── --list-models ──────────────────────────────────────────────────────────
	if args.ListModels != "" {
		printModelList(authStorage)
		return
	}

	// ── @file attachment processing ────────────────────────────────────────────
	var fileText string
	var fileImages []ai.ImageContent
	if len(args.FileArgs) > 0 {
		processed, err := processFileArguments(args.FileArgs, ProcessFileOptions{AutoResizeImages: true})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing file arguments: %v\n", err)
			os.Exit(1)
		}
		fileText = processed.Text
		fileImages = processed.Images
	}

	// ── Build RuntimeConfig ────────────────────────────────────────────────────
	cfg := &RuntimeConfig{
		Model:        model,
		SystemPrompt: buildSystemPrompt(args),
		APIKey:       apiKey,
		Provider:     model.Provider,
		CWD:          workDir,
		ExtDirs:      []string{codecore.GetExtensionsDir()},
		AuthStorage:  authStorage,
		OAuthStore:   oauthStore,
		Settings:     settingsManager,
		ThemeManager: themeManager,
	}

	// ── Mode dispatch ──────────────────────────────────────────────────────────
	mode := resolveMode(args)

	switch mode {
	case "interactive":
		im := NewInteractiveMode(cfg)
		if err := im.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "print":
		promptText := buildPrompt(args, fileText)
		if promptText == "" {
			// Try stdin
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				data, err := io.ReadAll(os.Stdin)
				if err == nil {
					promptText = strings.TrimSpace(string(data))
				}
			}
		}
		if promptText == "" {
			fmt.Fprintf(os.Stderr, "No prompt provided; pass text as arguments, @files, or pipe input\n")
			os.Exit(1)
		}
		if err := runPrintMode(cfg, promptText, fileImages); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "json":
		promptText := buildPrompt(args, fileText)
		if err := runJSONMode(cfg, promptText, fileImages); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "rpc":
		if len(args.FileArgs) > 0 {
			fmt.Fprintf(os.Stderr, "Error: @file arguments are not supported in RPC mode\n")
			os.Exit(1)
		}
		if err := runRPCMode(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", mode)
		os.Exit(1)
	}
}

// ============================================================================
// Mode resolution
// ============================================================================

func resolveMode(args Args) string {
	if args.Mode != "" {
		return args.Mode
	}
	if args.Print {
		return "print"
	}
	// If stdin is not a terminal (piped) go to print mode.
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		return "print"
	}
	return "interactive"
}

// ============================================================================
// Prompt builder
// ============================================================================

func buildPrompt(args Args, fileText string) string {
	var parts []string
	if fileText != "" {
		parts = append(parts, fileText)
	}
	if len(args.Messages) > 0 {
		parts = append(parts, strings.Join(args.Messages, " "))
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// ============================================================================
// System prompt builder
// ============================================================================

func buildSystemPrompt(args Args) string {
	base := args.SystemPrompt
	if base == "" {
		base = codecore.DefaultSystemPrompt
	}
	if len(args.AppendSystemPrompt) > 0 {
		base = base + "\n" + strings.Join(args.AppendSystemPrompt, "\n")
	}
	return base
}

// ============================================================================
// Package subcommand detection
// ============================================================================

// detectPackageSubcommand returns the subcommand if the first positional arg
// is a package management verb (install / uninstall / remove / update / list / info).
func detectPackageSubcommand(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue // skip flags
		}
		switch arg {
		case "install", "i", "add",
			"uninstall", "remove", "rm",
			"update", "upgrade",
			"list", "ls",
			"info":
			return arg
		}
		// First non-flag argument is not a subcommand.
		break
	}
	return ""
}

// ============================================================================
// config subcommand
// ============================================================================

func runConfigMode() {
	// In the Go implementation we open the settings file in the user's editor.
	rhoDir := defaultRhoDir()
	settingsPath := filepath.Join(rhoDir, "settings", "settings.json")
	fmt.Printf("rho configuration file: %s\n", settingsPath)
	fmt.Println("Edit this file with your text editor to configure rho.")
}

// ============================================================================
// Default directories
// ============================================================================

func defaultRhoDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".rho")
}

// ============================================================================
// Usage
// ============================================================================

func printUsage() {
	fmt.Print(`rho - A lightweight extensible TUI coding agent

Usage:
  rho [flags] [prompt] [@files...]
  rho install <source>
  rho remove <source>
  rho update [source]
  rho list
  rho info <source>
  rho config

Flags:
  --help, -h               Show this help message
  --version, -v            Show version
  --mode <mode>            Run mode: interactive (default), print, json, rpc
  --model <name>           Model to use (e.g., "claude-sonnet-4-20250514", "gpt-4o")
  --provider <name>        Provider to use (e.g., "anthropic", "openai", "google")
  --api-key <key>          API key for the provider
  --system-prompt <text>   Override system prompt
  --append-system-prompt   Append text to system prompt (repeatable)
  --thinking <level>       Thinking level: off, minimal, low, medium, high, xhigh
  --continue, -c           Continue the most recent session
  --resume, -r             Pick a session to resume
  --session <id>           Load a specific session
  --no-session             Disable session persistence
  --print, -p              Print mode: output response to stdout
  --list-models            List available models
  --tools <names>          Comma-separated list of tools to enable
  --no-tools               Disable all tools
  --no-builtin-tools       Disable built-in tools only
  --extension, -e <path>   Load extra extension (repeatable)
  --no-extensions          Disable all extensions
  --verbose                Verbose output

@file arguments:
  Pass @path/to/file to include a file (image or text) in the prompt.

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
  rho "List all Go files"
  rho --model gpt-4o --provider openai "Hello"
  rho --print "Summarise this" @notes.txt
  rho --mode rpc
  rho install npm:@foo/bar
  rho list
`)
}

// ============================================================================
// Model resolution
// ============================================================================

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

	// If model has a provider prefix like "anthropic/claude-sonnet-4", parse it.
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

	// Fallback: construct model from parts.
	api := ai.APIOpenAICompletions
	prov := ai.Provider(providerName)
	if prov == "" {
		prov = ai.ProviderOpenAI
	}

	// Auto-detect API from provider.
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
		// Prefer Anthropic models first, then OpenAI, then first available.
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
		// Fallback to first available.
		m := available[0]
		return ai.Model{
			API:      m.API,
			Provider: m.Provider,
			Name:     m.Name,
			BaseURL:  m.BaseURL,
		}
	}
	// No models with configured auth available — return empty model.
	return ai.Model{}
}

// providerHasAuth checks if a provider has configured authentication via any source.
func providerHasAuth(provider ai.Provider, authStorage *auth.AuthStorage, oauthStore *auth.OAuthStore) bool {
	if authStorage != nil {
		if authStorage.HasAPIKey(string(provider)) {
			return true
		}
	}
	if oauthStore != nil {
		if oauthStore.HasProvider(string(provider)) {
			return true
		}
	}
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
			// Check if OAuth credentials need refresh.
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
	for _, envKey := range ai.ProviderEnvKeys(provider) {
		if val := os.Getenv(envKey); val != "" {
			return val
		}
	}
	if authStorage != nil {
		if val, ok := authStorage.GetAPIKey(string(provider)); ok && strings.TrimSpace(val) != "" {
			return val
		}
	}
	return ""
}

func printModelList(authStorage *auth.AuthStorage) {
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
		provider := provider
		go func() {
			defer wg.Done()
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

// ============================================================================
// Run modes
// ============================================================================

func runPrintMode(cfg *RuntimeConfig, promptText string, images []ai.ImageContent) error {
	results, streamed, err := runAgentOnce(cfg, []agent.AgentMessage{{
		Role:    ai.RoleUser,
		Content: promptText,
		Images:  images,
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

func runJSONMode(cfg *RuntimeConfig, promptText string, images []ai.ImageContent) error {
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
		if prompt == "" {
			return fmt.Errorf("no prompt provided")
		}
		input.Messages = []agent.AgentMessage{{Role: ai.RoleUser, Content: prompt, Images: images}}
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
	rhoDir := defaultRhoDir()
	server := rpc.NewServer()
	server.SetModel(cfg.Model)
	server.SetAPIKey(cfg.APIKey)
	server.SetSystemPrompt(cfg.SystemPrompt)
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

// ============================================================================
// Config file (legacy helpers kept for interactive mode compatibility)
// ============================================================================

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
