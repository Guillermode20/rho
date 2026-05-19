// rho - A lightweight extensible TUI coding agent (Go translation of pi)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

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
	systemPrompt := flag.String("system-prompt", "You are a helpful coding assistant.", "System prompt for the agent")
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

	// Resolve model
	var model ai.Model
	if *modelName != "" {
		model = resolveModel(*modelName, *providerName)
	} else {
		model = getDefaultModel()
	}

	// Resolve API key
	if *apiKey == "" {
		*apiKey = resolveAPIKey(model)
	}

	// Resolve provider name
	if *providerName == "" {
		*providerName = string(model.Provider)
	}
	if model.Provider == "" {
		model.Provider = ai.Provider(*providerName)
	}

	if *listModels {
		printModelList()
		return
	}

	// Build runtime config
	cfg := &RuntimeConfig{
		Model:        model,
		SystemPrompt: *systemPrompt,
		APIKey:       *apiKey,
		Provider:     model.Provider,
		CWD:          workDir,
	}

	switch *mode {
	case "interactive":
		im := NewInteractiveMode(cfg)
		if err := im.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "print":
		runPrintMode(*modelName, *providerName, *apiKey, *systemPrompt, *prompt)
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
	}

	return ai.Model{
		API:      api,
		Provider: prov,
		Name:     modelName,
	}
}

func getDefaultModel() ai.Model {
	// Check environment for preferred model
	models := ai.DefaultModels()
	if len(models) > 0 {
		return ai.Model{
			API:      models[0].API,
			Provider: models[0].Provider,
			Name:     models[0].Name,
			BaseURL:  models[0].BaseURL,
		}
	}
	return ai.Model{
		API:      ai.APIAnthropicMessages,
		Provider: ai.ProviderAnthropic,
		Name:     "claude-sonnet-4-20250514",
	}
}

func resolveAPIKey(model ai.Model) string {
	switch model.Provider {
	case ai.ProviderAnthropic:
		return providers.GetEnvAPIKey("ANTHROPIC_API_KEY", "CLAUDE_API_KEY")
	case ai.ProviderOpenAI, ai.ProviderOpenAICodex:
		return providers.GetEnvAPIKey("OPENAI_API_KEY")
	case ai.ProviderGoogle:
		return providers.GetEnvAPIKey("GOOGLE_API_KEY", "GEMINI_API_KEY")
	case ai.ProviderDeepSeek:
		return providers.GetEnvAPIKey("DEEPSEEK_API_KEY")
	case ai.ProviderMistral:
		return providers.GetEnvAPIKey("MISTRAL_API_KEY")
	case ai.ProviderGroq:
		return providers.GetEnvAPIKey("GROQ_API_KEY")
	default:
		return providers.GetEnvAPIKey("OPENAI_API_KEY", "ANTHROPIC_API_KEY")
	}
}

func printModelList() {
	fmt.Println("Available models:")
	fmt.Println()
	for _, def := range ai.DefaultModels() {
		providerName := def.Provider
		reasoning := ""
		if def.Reasoning {
			reasoning = " [reasoning]"
		}
		fmt.Printf("  %s/%s  (%s)%s\n", providerName, def.Name, string(def.API), reasoning)
	}
}

func runPrintMode(modelName, providerName, apiKey, sysPrompt, promptText string) {
	if promptText == "" {
		args := flag.Args()
		if len(args) > 0 {
			promptText = strings.Join(args, " ")
		} else {
			// Read from stdin if piped
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				data := make([]byte, 1024*1024)
				n, _ := os.Stdin.Read(data)
				promptText = strings.TrimSpace(string(data[:n]))
			}
		}
		if promptText == "" {
			fmt.Fprintln(os.Stderr, "No prompt provided. Use -prompt, pass text as arguments, or pipe input.")
			os.Exit(1)
		}
	}

	model := resolveModel(modelName, providerName)

	_ = model
	fmt.Printf("User: %s\n", promptText)
	fmt.Println("---")
	fmt.Println("Configure an AI provider to get real responses.")
	fmt.Println("Set ANTHROPIC_API_KEY, OPENAI_API_KEY, or GOOGLE_API_KEY environment variables.")
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
