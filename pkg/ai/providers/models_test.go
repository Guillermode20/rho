package providers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/earendil-works/rho/pkg/ai"
)

func TestResolveBaseURL(t *testing.T) {
	// Test default models lookup
	url := ResolveBaseURL(ai.ProviderGroq)
	if url != "https://api.groq.com/openai/v1" {
		t.Errorf("expected groq base url to be https://api.groq.com/openai/v1, got %s", url)
	}

	// Test fallbacks
	url = ResolveBaseURL(ai.ProviderAnthropic)
	if url != "https://api.anthropic.com" {
		t.Errorf("expected anthropic default, got %s", url)
	}

	// Test environment variable override
	os.Setenv("MISTRAL_BASE_URL", "https://custom.mistral.ai")
	defer os.Unsetenv("MISTRAL_BASE_URL")
	url = ResolveBaseURL(ai.ProviderMistral)
	if url != "https://custom.mistral.ai" {
		t.Errorf("expected custom mistral url, got %s", url)
	}
}

func TestGuessModelDefinition(t *testing.T) {
	// Test match with built-in model
	def := GuessModelDefinition(ai.ProviderAnthropic, "claude-3-5-sonnet-20241022")
	if def.Name != "claude-3-5-sonnet-20241022" {
		t.Errorf("expected matched name, got %s", def.Name)
	}

	// Test guessed reasoning model
	def = GuessModelDefinition(ai.ProviderCrof, "o3-mini-reasoning")
	if !def.Reasoning {
		t.Errorf("expected reasoning to be true for o3-mini-reasoning")
	}

	// Test guessed non-reasoning model
	def = GuessModelDefinition(ai.ProviderCrof, "gpt-4o-mini")
	if def.Reasoning {
		t.Errorf("expected reasoning to be false for gpt-4o-mini")
	}
}

func TestFetchModelsForProvider_OpenAI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{
			"data": [
				{"id": "gpt-4o"},
				{"id": "o1-preview"}
			]
		}`)
	}))
	defer server.Close()

	os.Setenv("CROF_BASE_URL", server.URL)
	defer os.Unsetenv("CROF_BASE_URL")

	defs, err := FetchModelsForProvider(ai.ProviderCrof, "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(defs) != 2 {
		t.Fatalf("expected 2 models, got %d", len(defs))
	}
	if defs[0].Name != "gpt-4o" || defs[1].Name != "o1-preview" {
		t.Errorf("unexpected models parsed: %+v", defs)
	}
}
