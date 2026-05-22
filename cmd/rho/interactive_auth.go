package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/earendil-works/rho/pkg/agent/auth"
	"github.com/earendil-works/rho/pkg/agent/ui"
	"github.com/earendil-works/rho/pkg/ai"
	"github.com/earendil-works/rho/pkg/tui"
)

func (im *InteractiveMode) hasOAuth() bool {
	if im.config.OAuthStore != nil {
		return im.config.OAuthStore.HasProvider(string(im.config.Provider))
	}
	return false
}

func (im *InteractiveMode) handlePendingLogin(value string) {
	provider := im.pendingLoginProvider
	im.pendingLoginProvider = ""
	im.ui.ClearInput()

	if value == "/cancel" {
		im.addSystemMessage(fmt.Sprintf("Login cancelled for %s.", provider))
		return
	}
	if im.config.AuthStorage == nil {
		im.addSystemMessage("No auth storage is configured.")
		return
	}
	key := strings.TrimSpace(value)
	if key == "" {
		im.addSystemMessage(fmt.Sprintf("No API key saved for %s.", provider))
		return
	}
	if err := im.config.AuthStorage.SetAPIKey(provider, key); err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not save API key for %s: %v", provider, err))
		return
	}
	if string(im.config.Provider) == provider {
		im.config.APIKey = key
	}
	im.addSystemMessage(fmt.Sprintf("Saved API key for %s in %s.", provider, shortenPath(defaultAuthKeysPath())))
	if im.coordinator != nil && im.coordinator.Services != nil && im.coordinator.Services.ModelReg != nil {
		im.coordinator.Services.ModelReg.FetchModelsAsync(ai.Provider(provider))
	}
}

func (im *InteractiveMode) handleLoginCommand(args []string) {
	im.ui.ClearInput()
	if im.config.AuthStorage == nil {
		im.addSystemMessage("No auth storage is configured.")
		return
	}

	if len(args) == 0 {
		im.showLoginAuthTypeSelector()
		return
	}

	provider := strings.ToLower(args[0])
	if !im.isKnownProvider(provider) {
		im.addSystemMessage(fmt.Sprintf("Unknown provider: %s\nAvailable providers: %s", provider, strings.Join(im.availableProviderNames(), ", ")))
		return
	}

	if len(args) > 1 {
		key := strings.TrimSpace(strings.Join(args[1:], " "))
		if err := im.config.AuthStorage.SetAPIKey(provider, key); err != nil {
			im.addSystemMessage(fmt.Sprintf("Could not save API key for %s: %v", provider, err))
			return
		}
		if string(im.config.Provider) == provider {
			im.config.APIKey = key
		}
		im.addSystemMessage(fmt.Sprintf("Saved API key for %s in %s.", provider, shortenPath(defaultAuthKeysPath())))
		if im.coordinator != nil && im.coordinator.Services != nil && im.coordinator.Services.ModelReg != nil {
			im.coordinator.Services.ModelReg.FetchModelsAsync(ai.Provider(provider))
		}
		return
	}

	im.pendingLoginProvider = provider
	im.addSystemMessage(fmt.Sprintf("Paste API key for %s. It will be stored in %s. Enter /cancel to abort.", provider, shortenPath(defaultAuthKeysPath())))
}

func (im *InteractiveMode) showLoginAuthTypeSelector() {
	items := []ui.AutocompleteItem{
		{Value: "api-key", Label: "API key", Description: "Paste and store a provider API key"},
		{Value: "oauth", Label: "OAuth", Description: "Choose an OAuth-capable provider"},
	}
	im.ui.OpenModalSelector("Login method", items, func(item ui.AutocompleteItem) {
		switch item.Value {
		case "api-key":
			im.showProviderSelector("Login provider", "login")
		case "oauth":
			im.showOAuthProviderSelector()
		}
	}, func() {
		im.addSystemMessage("Login cancelled.")
	})
}

func (im *InteractiveMode) showOAuthProviderSelector() {
	options := ai.GetOAuthProviders()
	if len(options) == 0 {
		im.addSystemMessage("No OAuth providers are registered.")
		return
	}
	items := make([]ui.AutocompleteItem, 0, len(options))
	for _, option := range options {
		items = append(items, ui.AutocompleteItem{
			Value:       string(option.ProviderID),
			Label:       option.Name,
			Description: option.Description,
		})
	}
	im.ui.OpenModalSelector("OAuth provider", items, func(item ui.AutocompleteItem) {
		provider := ai.OAuthProviderFactory(ai.OAuthProviderID(item.Value))
		if provider == nil || provider.AuthInfo() == nil {
			im.addSystemMessage(fmt.Sprintf("OAuth provider %s is not available.", item.Value))
			return
		}
		info := provider.AuthInfo()
		im.addSystemMessage(fmt.Sprintf("OAuth login for %s is not fully automated in this TUI yet.\nAuthorization URL: %s\nUse /login %s <api-key> for API-key auth.", item.Value, info.AuthURL, item.Value))
	}, func() {
		im.addSystemMessage("OAuth login cancelled.")
	})
}

func (im *InteractiveMode) handleLogoutCommand(args []string) {
	im.ui.ClearInput()
	if im.config.AuthStorage == nil {
		im.addSystemMessage("No auth storage is configured.")
		return
	}
	if len(args) == 0 {
		im.showProviderSelector("Logout provider", "logout")
		return
	}
	provider := strings.ToLower(args[0])
	if err := im.config.AuthStorage.DeleteAPIKey(provider); err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not remove API key for %s: %v", provider, err))
		return
	}
	if string(im.config.Provider) == provider {
		im.config.APIKey = resolveAPIKey(im.config.Model, im.config.AuthStorage)
	}
	im.addSystemMessage(fmt.Sprintf("Removed saved API key for %s.", provider))
	if im.coordinator != nil && im.coordinator.Services != nil && im.coordinator.Services.ModelReg != nil {
		im.coordinator.Services.ModelReg.ResetProviderModels(ai.Provider(provider))
	}
}

func (im *InteractiveMode) startOAuthLogin(providerID ai.OAuthProviderID) {
	if im.config.OAuthStore == nil {
		im.addSystemMessage("No OAuth storage is configured.")
		return
	}
	provider, ok := ai.OAuthProviderFactory(providerID).(*ai.OAuthProvider)
	if !ok || provider == nil || provider.AuthInfo() == nil {
		im.addSystemMessage(fmt.Sprintf("OAuth provider %s is not available.", providerID))
		return
	}
	authURL, pkce, err := provider.NewAuthorizationURL()
	if err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not start OAuth login for %s: %v", providerID, err))
		return
	}

	callbacks, stop, err := startOAuthCallbackServer(provider.AuthInfo().RedirectURI)
	if err != nil {
		callbacks = nil
		im.addSystemMessage(fmt.Sprintf("Could not listen for OAuth callback: %v\nPaste the redirected URL or authorization code manually.", err))
	}

	var once sync.Once
	finish := func(code string) {
		once.Do(func() {
			if stop != nil {
				stop()
			}
			if im.canSendUI() {
				im.program.Send(uiClosePromptMsg{})
			} else {
				im.ui.CloseModal()
			}
			go im.exchangeAndStoreOAuthCode(providerID, code, pkce)
		})
	}
	if callbacks != nil {
		go func() {
			select {
			case code := <-callbacks:
				if im.canSendUI() {
					im.program.Send(tui.AgentStatusMsg{Text: im.statusText("OAuth callback received")})
				}
				finish(code)
			case <-time.After(5 * time.Minute):
				if stop != nil {
					stop()
				}
				if im.canSendUI() {
					im.program.Send(tui.AddMessageMsg{
						Role:    string(ai.RoleAssistant),
						Content: fmt.Sprintf("OAuth login for %s timed out.", providerID),
						Model:   "rho",
					})
				}
			}
		}()
	}

	openErr := im.openOAuthURL(authURL)
	message := fmt.Sprintf("OAuth login for %s started.\nAuthorization URL: %s\nPaste the redirected URL or authorization code below if the browser callback is not captured.", provider.AuthInfo().Name, authURL)
	if openErr != nil {
		message += fmt.Sprintf("\nCould not open browser automatically: %v", openErr)
	}
	im.addSystemMessage(message)
	im.ui.OpenModalPrompt("OAuth callback or code", "Paste redirect URL or authorization code", func(value string) {
		code, err := extractOAuthCode(value)
		if err != nil {
			im.addSystemMessage(fmt.Sprintf("OAuth login cancelled: %v", err))
			if stop != nil {
				stop()
			}
			return
		}
		finish(code)
	}, func() {
		if stop != nil {
			stop()
		}
		im.addSystemMessage("OAuth login cancelled.")
	})
}

func (im *InteractiveMode) exchangeAndStoreOAuthCode(providerID ai.OAuthProviderID, code string, pkce *ai.PKCE) {
	if strings.TrimSpace(code) == "" {
		im.postSystemMessage("OAuth login cancelled: no authorization code provided.")
		return
	}
	if im.canSendUI() {
		im.program.Send(tui.AgentStatusMsg{Text: im.statusText("Completing OAuth login...")})
	}
	exchange := im.config.OAuthExchange
	if exchange == nil {
		exchange = func(providerID ai.OAuthProviderID, code string, pkce *ai.PKCE) (*ai.OAuthCredentials, error) {
			provider, ok := ai.OAuthProviderFactory(providerID).(*ai.OAuthProvider)
			if !ok || provider == nil {
				return nil, fmt.Errorf("OAuth provider %s is not available", providerID)
			}
			return provider.ExchangeCode(code, pkce)
		}
	}
	creds, err := exchange(providerID, code, pkce)
	if err != nil {
		im.postSystemMessage(fmt.Sprintf("OAuth login failed for %s: %v", providerID, err))
		if im.canSendUI() {
			im.program.Send(tui.AgentStatusMsg{Text: im.statusText("")})
		}
		return
	}
	if creds == nil || strings.TrimSpace(creds.AccessToken) == "" {
		im.postSystemMessage(fmt.Sprintf("OAuth login failed for %s: no access token returned.", providerID))
		return
	}
	if err := im.config.OAuthStore.Save(&auth.OAuthCredential{
		Provider:     string(providerID),
		AccessToken:  creds.AccessToken,
		RefreshToken: creds.RefreshToken,
		ExpiresAt:    creds.ExpiresAt,
		Scopes:       creds.Scopes,
		TokenType:    creds.TokenType,
	}); err != nil {
		im.postSystemMessage(fmt.Sprintf("Could not save OAuth credentials for %s: %v", providerID, err))
		return
	}
	if string(im.config.Provider) == string(providerID) {
		im.config.APIKey = creds.AccessToken
	}
	if im.canSendUI() {
		im.program.Send(tui.AgentStatusMsg{Text: im.statusText("")})
	}
	im.postSystemMessage(fmt.Sprintf("Saved OAuth credentials for %s in %s.", providerID, shortenPath(defaultOAuthPath())))
}

func (im *InteractiveMode) openOAuthURL(rawURL string) error {
	if im.config.OpenURL != nil {
		return im.config.OpenURL(rawURL)
	}
	return openURLInBrowser(rawURL)
}

func extractOAuthCode(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("empty OAuth response")
	}
	if strings.Contains(value, "://") {
		u, err := url.Parse(value)
		if err != nil {
			return "", fmt.Errorf("could not parse URL: %w", err)
		}
		if code := u.Query().Get("code"); code != "" {
			return code, nil
		}
		return "", fmt.Errorf("no code parameter in URL")
	}
	return value, nil
}

func startOAuthCallbackServer(redirectURI string) (<-chan string, func(), error) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return nil, nil, err
	}
	listener, err := net.Listen("tcp", u.Host)
	if err != nil {
		return nil, nil, err
	}
	codeCh := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc(u.Path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			http.Error(w, "missing OAuth code", http.StatusBadRequest)
			return
		}
		select {
		case codeCh <- code:
		default:
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><title>rho login complete</title><p>rho login complete. You can close this tab.</p>"))
	})
	server := &http.Server{Handler: mux}
	go func() {
		_ = server.Serve(listener)
	}()
	stop := func() {
		_ = server.Close()
	}
	return codeCh, stop, nil
}

func openURLInBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}
