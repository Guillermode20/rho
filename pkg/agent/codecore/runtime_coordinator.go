// Package codecore provides the core runtime coordination layer.
// RuntimeCoordinator centralizes model/auth/settings/session/theme logic
// so that the Bubble Tea interactive mode stays a thin presentation layer.
package codecore

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/auth"
	agenttheme "github.com/earendil-works/rho/pkg/agent/theme"
	"github.com/earendil-works/rho/pkg/ai"
)

// RuntimeCoordinator centralizes model, auth, settings, session, and theme
// coordination. InteractiveMode delegates to this struct for all business logic.
type RuntimeCoordinator struct {
	mu sync.RWMutex

	// Core services
	Services    *AgentSessionServices
	OAuthStore  *auth.OAuthStore
	AuthStorage *auth.AuthStorage
	SessionMgr  *agent.SessionManager
	Settings    *SettingsManager
	ThemeMgr    *agenttheme.ThemeManager

	// Current state
	Model     ai.Model
	Provider  ai.Provider
	APIKey    string
	SessionID string
	CWD       string

	// Extension statuses
	ExtensionStatuses map[string]string

	// Callbacks for UI updates (set by the presentation layer)
	OnStatusUpdate func(text string)
}

// NewRuntimeCoordinator creates a new runtime coordinator.
func NewRuntimeCoordinator(
	services *AgentSessionServices,
	oauthStore *auth.OAuthStore,
	authStorage *auth.AuthStorage,
	themeMgr *agenttheme.ThemeManager,
	settings *SettingsManager,
	sessionMgr *agent.SessionManager,
	model ai.Model,
	provider ai.Provider,
	apiKey string,
	cwd string,
	sessionID string,
) *RuntimeCoordinator {
	return &RuntimeCoordinator{
		Services:          services,
		OAuthStore:        oauthStore,
		AuthStorage:       authStorage,
		ThemeMgr:          themeMgr,
		Settings:          settings,
		SessionMgr:        sessionMgr,
		Model:             model,
		Provider:          provider,
		APIKey:            apiKey,
		SessionID:         sessionID,
		CWD:               cwd,
		ExtensionStatuses: make(map[string]string),
	}
}

// ─── Status ─────────────────────────────────────────────────────────────────

// StatusText builds the status line text.
func (rc *RuntimeCoordinator) StatusText(activity string) string {
	parts := []string{
		"rho",
		fmt.Sprintf("%s/%s", rc.Provider, rc.Model.Name),
	}
	if name := rc.sessionNameUnsafe(rc.SessionID); name != "" {
		parts = append(parts, name)
	}
	if activity != "" {
		parts = append(parts, activity)
	} else {
		parts = append(parts, shortenPath(rc.CWD))
	}
	for _, text := range rc.ExtensionStatuses {
		if strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " | ")
}

// UpdateStatus triggers the status update callback if set.
func (rc *RuntimeCoordinator) UpdateStatus(activity string) {
	if rc.OnStatusUpdate != nil {
		rc.OnStatusUpdate(rc.StatusText(activity))
	}
}

// ─── Model ───────────────────────────────────────────────────────────────────

// SelectModel selects a model and resolves its auth token.
func (rc *RuntimeCoordinator) SelectModel(modelDef ai.ModelDefinition) string {
	rc.mu.Lock()
	rc.Model = ai.Model{
		API:      modelDef.API,
		Provider: modelDef.Provider,
		Name:     modelDef.Name,
		BaseURL:  modelDef.BaseURL,
	}
	rc.Provider = modelDef.Provider
	rc.APIKey = rc.resolveAuthToken()
	rc.mu.Unlock()
	rc.UpdateStatus("")
	return fmt.Sprintf("%s/%s", modelDef.Provider, modelDef.Name)
}

// CurrentModelInfo returns the current model/provider string.
func (rc *RuntimeCoordinator) CurrentModelInfo() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return fmt.Sprintf("%s/%s", rc.Provider, rc.Model.Name)
}

// ─── Auth ────────────────────────────────────────────────────────────────────

// AvailableProviderNames returns a sorted list of known provider names.
func (rc *RuntimeCoordinator) AvailableProviderNames() []string {
	seen := make(map[string]bool)
	var names []string
	for _, model := range ai.DefaultModels() {
		name := string(model.Provider)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// IsKnownProvider checks if a provider name is recognized.
func (rc *RuntimeCoordinator) IsKnownProvider(provider string) bool {
	for _, p := range rc.AvailableProviderNames() {
		if p == provider {
			return true
		}
	}
	return false
}

// SaveAPIKey saves an API key for a provider.
func (rc *RuntimeCoordinator) SaveAPIKey(provider, key string) error {
	if rc.AuthStorage == nil {
		return fmt.Errorf("no auth storage configured")
	}
	if err := rc.AuthStorage.SetAPIKey(provider, key); err != nil {
		return fmt.Errorf("could not save API key: %w", err)
	}
	rc.mu.Lock()
	if string(rc.Provider) == provider {
		rc.APIKey = key
	}
	rc.mu.Unlock()
	rc.UpdateStatus("")
	return nil
}

// RemoveCredentials removes both API keys and OAuth credentials for a provider.
// Returns true if anything was removed.
func (rc *RuntimeCoordinator) RemoveCredentials(provider string) bool {
	deletedAny := false
	if rc.AuthStorage != nil {
		if rc.AuthStorage.HasAPIKey(provider) {
			_ = rc.AuthStorage.DeleteAPIKey(provider)
			deletedAny = true
		}
	}
	if rc.OAuthStore != nil {
		if rc.OAuthStore.HasProvider(provider) {
			_ = rc.OAuthStore.Delete(provider)
			deletedAny = true
		}
	}
	if deletedAny {
		rc.mu.Lock()
		if string(rc.Provider) == provider {
			rc.APIKey = rc.resolveAuthToken()
		}
		rc.mu.Unlock()
		rc.UpdateStatus("")
	}
	return deletedAny
}

// HasOAuthCredentials checks if a provider has stored OAuth credentials.
func (rc *RuntimeCoordinator) HasOAuthCredentials(provider string) bool {
	if rc.OAuthStore == nil {
		return false
	}
	return rc.OAuthStore.HasProvider(provider)
}

// GetOAuthStatus returns a human-readable OAuth status for a provider.
func (rc *RuntimeCoordinator) GetOAuthStatus(provider string) string {
	if rc.OAuthStore == nil {
		return ""
	}
	cred, ok := rc.OAuthStore.Get(provider)
	if !ok {
		return ""
	}
	if cred.ExpiresAt > 0 {
		remaining := cred.ExpiresAt - time.Now().Unix()
		if remaining > 0 {
			return fmt.Sprintf("OAuth (expires in %dh)", remaining/3600)
		}
		return "OAuth (expired)"
	}
	return "OAuth"
}

// ExchangeAndStoreOAuthCode exchanges an authorization code for tokens and saves them.
func (rc *RuntimeCoordinator) ExchangeAndStoreOAuthCode(providerID ai.OAuthProviderID, code string, pkce *ai.PKCE, exchange func(ai.OAuthProviderID, string, *ai.PKCE) (*ai.OAuthCredentials, error)) (*ai.OAuthCredentials, error) {
	if code == "" {
		return nil, fmt.Errorf("no authorization code provided")
	}

	var creds *ai.OAuthCredentials
	var err error
	if exchange != nil {
		creds, err = exchange(providerID, code, pkce)
	} else {
		provider, ok := ai.OAuthProviderFactory(providerID).(*ai.OAuthProvider)
		if !ok || provider == nil {
			return nil, fmt.Errorf("OAuth provider %s is not available", providerID)
		}
		creds, err = provider.ExchangeCode(code, pkce)
	}
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	if creds == nil || strings.TrimSpace(creds.AccessToken) == "" {
		return nil, fmt.Errorf("no access token returned")
	}

	// Save credentials
	if rc.OAuthStore == nil {
		return nil, fmt.Errorf("no OAuth storage configured")
	}
	if err := rc.OAuthStore.Save(&auth.OAuthCredential{
		Provider:     string(providerID),
		AccessToken:  creds.AccessToken,
		RefreshToken: creds.RefreshToken,
		ExpiresAt:    creds.ExpiresAt,
		Scopes:       creds.Scopes,
		TokenType:    creds.TokenType,
	}); err != nil {
		return nil, fmt.Errorf("could not save credentials: %w", err)
	}

	rc.mu.Lock()
	if string(rc.Provider) == string(providerID) {
		rc.APIKey = creds.AccessToken
		// Verify model availability
		token := rc.resolveAuthToken()
		if token != "" {
			rc.APIKey = token
		}
	}
	rc.mu.Unlock()
	rc.UpdateStatus("Credentials verified ✓")

	return creds, nil
}

// resolveAuthToken resolves the best available auth token (API key or OAuth).
func (rc *RuntimeCoordinator) resolveAuthToken() string {
	provider := string(rc.Model.Provider)

	// Check API keys
	if rc.AuthStorage != nil {
		if key, ok := rc.AuthStorage.GetAPIKey(provider); ok && strings.TrimSpace(key) != "" {
			return key
		}
	}

	// Check OAuth credentials with refresh
	if rc.OAuthStore != nil {
		if cred, ok := rc.OAuthStore.Get(provider); ok && strings.TrimSpace(cred.AccessToken) != "" {
			goCreds := &ai.OAuthCredentials{
				AccessToken:  cred.AccessToken,
				RefreshToken: cred.RefreshToken,
				ExpiresAt:    cred.ExpiresAt,
				ProviderID:   cred.Provider,
			}
			p := ai.OAuthProviderFactory(ai.OAuthProviderID(cred.Provider))
			refreshed, err := ai.RefreshIfNeeded(goCreds, p)
			if err == nil && refreshed != goCreds && strings.TrimSpace(refreshed.AccessToken) != "" {
				// Save refreshed
				_ = rc.OAuthStore.Save(&auth.OAuthCredential{
					Provider:     refreshed.ProviderID,
					AccessToken:  refreshed.AccessToken,
					RefreshToken: refreshed.RefreshToken,
					ExpiresAt:    refreshed.ExpiresAt,
				})
				return refreshed.AccessToken
			}
			return cred.AccessToken
		}
	}

	return ""
}

// ─── Settings ────────────────────────────────────────────────────────────────

// GetSetting returns a setting value by key.
func (rc *RuntimeCoordinator) GetSetting(key string) interface{} {
	if rc.Settings == nil {
		return nil
	}
	return rc.Settings.Get(key)
}

// SetSetting sets a setting value.
func (rc *RuntimeCoordinator) SetSetting(key string, value interface{}) {
	if rc.Settings != nil {
		_ = rc.Settings.SetUser(key, value)
	}
}

// SettingBool returns a setting as bool with a fallback.
func (rc *RuntimeCoordinator) SettingBool(key string, fallback bool) bool {
	if rc.Settings == nil {
		return fallback
	}
	if val := rc.Settings.Get(key); val != nil {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return fallback
}

// SettingString returns a setting as string with a fallback.
func (rc *RuntimeCoordinator) SettingString(key, fallback string) string {
	if rc.Settings == nil {
		return fallback
	}
	val := rc.Settings.Get(key)
	if val == nil {
		return fallback
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fallback
}

// ─── Theme ───────────────────────────────────────────────────────────────────

// ActiveThemeName returns the name of the currently active theme.
func (rc *RuntimeCoordinator) ActiveThemeName() string {
	if rc.ThemeMgr == nil {
		return ""
	}
	return rc.ThemeMgr.ActiveName()
}

// SelectTheme activates and persists a theme.
func (rc *RuntimeCoordinator) SelectTheme(name string) error {
	if rc.ThemeMgr == nil {
		return fmt.Errorf("no theme manager configured")
	}
	if err := rc.ThemeMgr.SetActive(name); err != nil {
		return err
	}
	if rc.Settings != nil {
		_ = rc.Settings.SetUser("theme", name)
	}
	rc.UpdateStatus("")
	return nil
}

// ─── Session ─────────────────────────────────────────────────────────────────

// SaveCurrentSession persists the current session.
func (rc *RuntimeCoordinator) SaveCurrentSession(messages []agent.AgentMessage) error {
	if rc.SessionMgr == nil {
		return nil
	}
	rc.mu.RLock()
	header := agent.SessionHeader{
		ID:        rc.SessionID,
		Timestamp: time.Now().Format(time.RFC3339),
		CWD:       rc.CWD,
	}
	rc.mu.RUnlock()
	return rc.SessionMgr.Save(rc.SessionID, header, messages)
}

// ResumeSession loads and returns a session's messages.
func (rc *RuntimeCoordinator) ResumeSession(sessionID string) (agent.SessionHeader, []agent.AgentMessage, error) {
	if rc.SessionMgr == nil {
		return agent.SessionHeader{}, nil, fmt.Errorf("no session manager configured")
	}
	header, messages, err := rc.SessionMgr.Load(sessionID)
	if err != nil {
		return agent.SessionHeader{}, nil, err
	}

	rc.mu.Lock()
	rc.SessionID = sessionID
	if header.CWD != "" {
		rc.CWD = header.CWD
	}
	rc.mu.Unlock()
	rc.UpdateStatus("")

	return header, messages, nil
}

// StartNewSession begins a new session.
func (rc *RuntimeCoordinator) StartNewSession() string {
	newID := agent.CurrentSessionID()
	rc.mu.Lock()
	rc.SessionID = newID
	rc.mu.Unlock()
	rc.UpdateStatus("")
	return newID
}

// ForkSession forks the current session.
func (rc *RuntimeCoordinator) ForkSession(messages []agent.AgentMessage) (newID string, err error) {
	if rc.SessionMgr == nil {
		return "", fmt.Errorf("no session manager configured")
	}
	parentID := rc.SessionID
	newID = agent.CurrentSessionID()
	rc.mu.Lock()
	rc.SessionID = newID
	rc.mu.Unlock()

	header := agent.SessionHeader{
		ID:            newID,
		Timestamp:     time.Now().Format(time.RFC3339),
		CWD:           rc.CWD,
		ParentSession: parentID,
	}
	if err := rc.SessionMgr.Save(newID, header, messages); err != nil {
		return "", fmt.Errorf("could not fork session: %w", err)
	}
	rc.UpdateStatus("")
	return newID, nil
}

// ListSessions returns all saved sessions.
func (rc *RuntimeCoordinator) ListSessions() ([]agent.SessionInfo, error) {
	if rc.SessionMgr == nil {
		return nil, fmt.Errorf("no session manager configured")
	}
	return rc.SessionMgr.List()
}

// sessionName reads a session name without acquiring any lock.
// Caller must hold rc.mu if concurrent access is possible.
func (rc *RuntimeCoordinator) sessionName(sessionID string) string {
	return rc.sessionNameUnsafe(sessionID)
}

// sessionNameUnsafe reads a session name without any locking.
func (rc *RuntimeCoordinator) sessionNameUnsafe(sessionID string) string {
	names := rc.sessionNamesUnsafe()
	if names == nil {
		return ""
	}
	return names[sessionID]
}

// SessionName returns the display name for a session (thread-safe).
func (rc *RuntimeCoordinator) SessionName(sessionID string) string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.sessionNameUnsafe(sessionID)
}

// SetSessionName sets the display name for the current session.
func (rc *RuntimeCoordinator) SetSessionName(name string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	names := rc.sessionNamesUnsafe()
	if name == "" {
		delete(names, rc.SessionID)
	} else {
		names[rc.SessionID] = name
	}
	rc.setUserSettingUnsafe("sessionNames", names)
	rc.UpdateStatus("")
}

func (rc *RuntimeCoordinator) sessionNamesUnsafe() map[string]string {
	out := make(map[string]string)
	if rc.Settings == nil {
		return out
	}
	raw := rc.Settings.Get("sessionNames")
	switch vals := raw.(type) {
	case map[string]string:
		for k, v := range vals {
			out[k] = v
		}
	case map[string]interface{}:
		for k, v := range vals {
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
	}
	return out
}

func (rc *RuntimeCoordinator) setUserSettingUnsafe(key string, value interface{}) {
	if rc.Settings != nil {
		_ = rc.Settings.SetUser(key, value)
	}
}

// ─── Session Tree ────────────────────────────────────────────────────────────

// SessionTreeNode represents a node in the session tree.
type SessionTreeNode struct {
	ID        string
	Label     string
	Preview   string
	Timestamp string
	Depth     int
	Children  []*SessionTreeNode
}

// BuildSessionTree builds parent/child tree from saved session headers.
func (rc *RuntimeCoordinator) BuildSessionTree(allSessions []agent.SessionInfo) []*SessionTreeNode {
	if rc.SessionMgr == nil {
		return nil
	}

	childMap := make(map[string][]*SessionTreeNode)
	nodeMap := make(map[string]*SessionTreeNode)

	// First pass: create nodes
	for _, s := range allSessions {
		node := &SessionTreeNode{
			ID:        s.ID,
			Preview:   s.Preview,
			Timestamp: s.Timestamp,
			Label:     s.ID,
		}
		if len(s.ID) > 16 {
			node.Label = s.ID[:16] + "..."
		}
		if s.Preview != "" {
			preview := s.Preview
			if len(preview) > 30 {
				preview = preview[:30] + "..."
			}
			node.Label += " — " + preview
		}
		nodeMap[s.ID] = node
	}

	// Second pass: build parent/child relationships
	for _, s := range allSessions {
		header, _, err := rc.SessionMgr.Load(s.ID)
		if err != nil {
			continue
		}
		node := nodeMap[s.ID]
		if node == nil {
			continue
		}
		if header.ParentSession != "" {
			if parent, ok := nodeMap[header.ParentSession]; ok {
				parent.Children = append(parent.Children, node)
				childMap[header.ParentSession] = append(childMap[header.ParentSession], node)
			} else {
				childMap[""] = append(childMap[""], node)
			}
		} else {
			childMap[""] = append(childMap[""], node)
		}
	}

	roots := childMap[""]
	var assignDepth func(nodes []*SessionTreeNode, depth int)
	assignDepth = func(nodes []*SessionTreeNode, depth int) {
		for _, n := range nodes {
			n.Depth = depth
			assignDepth(n.Children, depth+1)
		}
	}
	assignDepth(roots, 0)
	return roots
}

// FlattenSessionTree flattens a tree into a linear list for display.
func FlattenSessionTree(roots []*SessionTreeNode) []*SessionTreeNode {
	var result []*SessionTreeNode
	var walk func(nodes []*SessionTreeNode)
	walk = func(nodes []*SessionTreeNode) {
		for _, n := range nodes {
			result = append(result, n)
			walk(n.Children)
		}
	}
	walk(roots)
	return result
}

// ─── Extension Status ────────────────────────────────────────────────────────

// SetExtensionStatus sets or clears an extension's status text.
func (rc *RuntimeCoordinator) SetExtensionStatus(key, text string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if text == "" {
		delete(rc.ExtensionStatuses, key)
	} else {
		rc.ExtensionStatuses[key] = text
	}
	rc.UpdateStatus("")
}

// GetExtensionStatuses returns a copy of extension statuses.
func (rc *RuntimeCoordinator) GetExtensionStatuses() map[string]string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	result := make(map[string]string, len(rc.ExtensionStatuses))
	for k, v := range rc.ExtensionStatuses {
		result[k] = v
	}
	return result
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// DefaultAuthKeysPath returns the default path for API key storage.
func DefaultAuthKeysPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".rho", "auth", "keys.json")
}

// DefaultOAuthPath returns the default path for OAuth credential storage.
func DefaultOAuthPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".rho", "auth", "oauth.json")
}

func shortenPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	if strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}
