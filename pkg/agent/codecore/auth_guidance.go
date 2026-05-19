package codecore

import "fmt"

func MissingAPIKeyMessage(provider string) string {
	switch provider {
	case "anthropic":
		return "No API key for Anthropic. Set ANTHROPIC_API_KEY or configure via ~/.rho/auth.json"
	case "openai":
		return "No API key for OpenAI. Set OPENAI_API_KEY or configure via ~/.rho/auth.json"
	case "google":
		return "No API key for Google AI. Set GOOGLE_API_KEY"
	case "deepseek":
		return "No API key for DeepSeek. Set DEEPSEEK_API_KEY"
	default:
		return fmt.Sprintf("No API key for %s. Set %s_API_KEY", provider, provider)
	}
}

func FormatNoModelsAvailableMessage(provider string) string {
	return fmt.Sprintf("No models available for %s. Run 'rho -list-models' to see available models.", provider)
}

func FormatAuthStatusMessage(available, missing []string) string {
	s := ""
	for _, a := range available {
		s += "✓ " + a + "\n"
	}
	for _, m := range missing {
		s += "✗ " + m + " (set " + m + "_API_KEY)\n"
	}
	return s
}
