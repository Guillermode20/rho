package codecore

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/auth"
	"github.com/earendil-works/rho/pkg/agent/extensions"
	"github.com/earendil-works/rho/pkg/ai"
)

// CreateAgentSessionServicesOptions configures service creation.
type CreateAgentSessionServicesOptions struct {
	RhoDir       string
	Model        ai.Model
	SystemPrompt string
	APIKey       string
	CWD          string
	ExtDirs      []string
}

// AgentSessionServices is a dependency injection container.
type AgentSessionServices struct {
	ModelReg    *ModelRegistry
	AuthStorage *auth.AuthStorage
	OAuthStore  *auth.OAuthStore
	SessionMgr  *agent.SessionManager
	Settings    *SettingsManager
	Extensions  *extensions.Runtime
	Diagnostics *DiagnosticsCollector
	Timings     *TimeTracker
	Telemetry   *TelemetryCollector
}

// NewAgentSessionServices creates and wires all session services.
func NewAgentSessionServices(opts CreateAgentSessionServicesOptions) (*AgentSessionServices, error) {
	rhoDir := opts.RhoDir
	if rhoDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot determine home directory: %w", err)
		}
		rhoDir = filepath.Join(home, ".rho")
	}

	dirs := []string{
		rhoDir,
		filepath.Join(rhoDir, "sessions"),
		filepath.Join(rhoDir, "extensions"),
		filepath.Join(rhoDir, "auth"),
		filepath.Join(rhoDir, "settings"),
		filepath.Join(rhoDir, "skills"),
		filepath.Join(rhoDir, "themes"),
		filepath.Join(rhoDir, "prompts"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, fmt.Errorf("cannot create directory %s: %w", d, err)
		}
	}

	modelReg := NewModelRegistry()
	for _, m := range ai.DefaultModels() {
		modelReg.RegisterModel(ai.Model{
			API:      m.API,
			Provider: m.Provider,
			Name:     m.Name,
			BaseURL:  m.BaseURL,
		})
	}

	authStorage := auth.NewAuthStorage(filepath.Join(rhoDir, "auth", "keys.json"))
	modelReg.SetAuthProvider(authStorage)
	oauthStore := auth.NewOAuthStore(filepath.Join(rhoDir, "auth", "oauth.json"))
	sessionMgr := agent.NewSessionManager(filepath.Join(rhoDir, "sessions"))
	settingsMgr := NewSettingsManager(filepath.Join(rhoDir, "settings"), "")
	diagnostics := NewDiagnosticsCollector()
	timings := NewTimeTracker()
	telemetry := NewTelemetryCollector(filepath.Join(rhoDir, "telemetry.jsonl"))

	extRuntime := extensions.NewRuntime()
	extDirs := opts.ExtDirs
	if len(extDirs) == 0 {
		extDirs = []string{filepath.Join(rhoDir, "extensions")}
	}
	if result := extensions.LoadExtensions(extDirs, extRuntime); len(result.Errors) > 0 {
		for _, err := range result.Errors {
			diagnostics.Report(&Diagnostic{
				Type:    DiagnosticTypeWarning,
				Message: fmt.Sprintf("Extension loading: %s", err),
				Scope:   "extensions",
			})
		}
	}

	return &AgentSessionServices{
		ModelReg:    modelReg,
		AuthStorage: authStorage,
		OAuthStore:  oauthStore,
		SessionMgr:  sessionMgr,
		Settings:    settingsMgr,
		Extensions:  extRuntime,
		Diagnostics: diagnostics,
		Timings:     timings,
		Telemetry:   telemetry,
	}, nil
}

// CreateAgentSessionServicesResult wraps session creation from services.
type CreateAgentSessionServicesResult struct {
	SessionID string
	Agent     *agent.AgentLoop
	Context   agent.AgentContext
}

// CreateAgentSessionFromServices creates a session using the services.
func (s *AgentSessionServices) CreateAgentSessionFromServices(opts CreateAgentSessionOptions) (*CreateAgentSessionServicesResult, error) {
	builder := NewSessionBuilder()
	result, err := builder.CreateAgentSession(opts)
	if err != nil {
		return nil, err
	}
	extTools := s.Extensions.GetCustomTools()
	if len(extTools) > 0 {
		result.Context.Tools = extensions.MergeTools(result.Context.Tools, extTools)
	}
	return &CreateAgentSessionServicesResult{
		SessionID: result.SessionID,
		Agent:     result.Agent,
		Context:   result.Context,
	}, nil
}
