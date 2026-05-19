package codecore_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/earendil-works/rho/pkg/agent/codecore"
	"github.com/earendil-works/rho/pkg/ai"
)

func TestNewAgentSessionServices(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "rho-test-*")
	defer os.RemoveAll(tmpDir)

	services, err := codecore.NewAgentSessionServices(codecore.CreateAgentSessionServicesOptions{
		RhoDir:      tmpDir,
		Model:       ai.Model{API: ai.APIAnthropicMessages, Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		SystemPrompt: "You are a helpful assistant.",
		CWD:         tmpDir,
	})

	if err != nil {
		t.Fatalf("NewAgentSessionServices failed: %v", err)
	}
	if services == nil {
		t.Fatal("expected non-nil services")
	}
}

func TestAgentSessionServicesFields(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "rho-test-*")
	defer os.RemoveAll(tmpDir)

	services, _ := codecore.NewAgentSessionServices(codecore.CreateAgentSessionServicesOptions{
		RhoDir:      tmpDir,
		Model:       ai.Model{API: ai.APIAnthropicMessages, Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		SystemPrompt: "Test prompt",
		CWD:         tmpDir,
	})

	if services.ModelReg == nil {
		t.Error("expected non-nil ModelReg")
	}
	if services.AuthStorage == nil {
		t.Error("expected non-nil AuthStorage")
	}
	if services.OAuthStore == nil {
		t.Error("expected non-nil OAuthStore")
	}
	if services.SessionMgr == nil {
		t.Error("expected non-nil SessionMgr")
	}
	if services.Settings == nil {
		t.Error("expected non-nil Settings")
	}
	if services.Extensions == nil {
		t.Error("expected non-nil Extensions")
	}
	if services.Diagnostics == nil {
		t.Error("expected non-nil Diagnostics")
	}
	if services.Timings == nil {
		t.Error("expected non-nil Timings")
	}
}

func TestAgentSessionServicesDirectories(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "rho-test-*")
	defer os.RemoveAll(tmpDir)

	_, _ = codecore.NewAgentSessionServices(codecore.CreateAgentSessionServicesOptions{
		RhoDir:      tmpDir,
		Model:       ai.Model{API: ai.APIAnthropicMessages, Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		SystemPrompt: "Test",
		CWD:         tmpDir,
	})

	for _, d := range []string{
		filepath.Join(tmpDir, "sessions"),
		filepath.Join(tmpDir, "extensions"),
	} {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			t.Errorf("directory not created: %s", d)
		}
	}
}
