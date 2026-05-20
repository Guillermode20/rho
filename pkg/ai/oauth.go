package ai

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuthProviderID identifies an OAuth provider.
type OAuthProviderID string

const (
	OAuthAnthropic     OAuthProviderID = "anthropic"
	OAuthGitHubCopilot OAuthProviderID = "github-copilot"
	OAuthOpenAICodex   OAuthProviderID = "openai-codex"
)

// OAuthCredentials stores OAuth tokens.
type OAuthCredentials struct {
	AccessToken  string `json:"accessToken"`
	Code         string `json:"code,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresAt    int64  `json:"expiresAt,omitempty"` // Unix timestamp
	ProviderID   string `json:"providerId"`
	Scopes       string `json:"scopes,omitempty"`
	TokenType    string `json:"tokenType,omitempty"`
}

// OAuthAuthInfo describes a provider's OAuth configuration.
type OAuthAuthInfo struct {
	ProviderID          OAuthProviderID `json:"providerId"`
	Name                string          `json:"name"`
	AuthURL             string          `json:"authUrl"`
	TokenURL            string          `json:"tokenUrl"`
	ClientID            string          `json:"clientId"`
	Scopes              []string        `json:"scopes"`
	RedirectURI         string          `json:"redirectUri"`
	PKCE                bool            `json:"pkce"`
	CodeChallengeMethod string          `json:"codeChallengeMethod,omitempty"`
}

// OAuthLoginCallbacks provides callbacks for the OAuth login flow.
type OAuthLoginCallbacks struct {
	OpenURL func(url string) error
	Poll    func() (*OAuthCredentials, error)
}

// OAuthSelectOption is an option in the OAuth provider selector.
type OAuthSelectOption struct {
	ProviderID  OAuthProviderID `json:"providerId"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	AuthInfo    *OAuthAuthInfo  `json:"authInfo"`
}

// OAuthSelectPrompt is shown to the user to pick an OAuth provider.
type OAuthSelectPrompt struct {
	Title   string              `json:"title"`
	Options []OAuthSelectOption `json:"options"`
}

// OAuthProviderInterface defines the interface for OAuth providers.
type OAuthProviderInterface interface {
	ProviderID() OAuthProviderID
	AuthInfo() *OAuthAuthInfo
	Login(callbacks OAuthLoginCallbacks) (*OAuthCredentials, error)
	Refresh(creds *OAuthCredentials) (*OAuthCredentials, error)
	// GetAPIKey converts stored credentials into an API key string.
	GetAPIKey(creds *OAuthCredentials) string
}

// PKCE generates PKCE challenge/verifier pairs.
type PKCE struct {
	Verifier        string
	Challenge       string
	ChallengeMethod string
}

// GeneratePKCE creates a PKCE challenge/verifier pair using S256.
func GeneratePKCE() (*PKCE, error) {
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("failed to generate verifier: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return &PKCE{
		Verifier:        verifier,
		Challenge:       challenge,
		ChallengeMethod: "S256",
	}, nil
}

// OAuthProvider is the base OAuth provider implementation.
type OAuthProvider struct {
	ID         OAuthProviderID
	Info       *OAuthAuthInfo
	httpClient *http.Client
}

// NewOAuthProvider creates a new OAuth provider.
func NewOAuthProvider(id OAuthProviderID) *OAuthProvider {
	p := &OAuthProvider{
		ID:         id,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}

	switch id {
	case OAuthAnthropic:
		p.Info = &OAuthAuthInfo{
			ProviderID:  OAuthAnthropic,
			Name:        "Anthropic",
			AuthURL:     "https://auth.anthropic.com/authorize",
			TokenURL:    "https://auth.anthropic.com/oauth/token",
			ClientID:    "pi-coding-agent",
			Scopes:      []string{"messages:write", "messages:read"},
			RedirectURI: "http://localhost:9876/callback",
			PKCE:        true,
		}
	case OAuthGitHubCopilot:
		p.Info = &OAuthAuthInfo{
			ProviderID:  OAuthGitHubCopilot,
			Name:        "GitHub Copilot",
			AuthURL:     "https://github.com/login/oauth/authorize",
			TokenURL:    "https://github.com/login/oauth/access_token",
			ClientID:    "Iv1.b697d80b5b83c5c7",
			Scopes:      []string{"read:user", "copilot"},
			RedirectURI: "http://localhost:9876/callback",
			PKCE:        false,
		}
	case OAuthOpenAICodex:
		p.Info = &OAuthAuthInfo{
			ProviderID:  OAuthOpenAICodex,
			Name:        "OpenAI Codex",
			AuthURL:     "https://github.com/login/oauth/authorize",
			TokenURL:    "https://github.com/login/oauth/access_token",
			ClientID:    "Iv1.b697d80b5b83c5c7",
			Scopes:      []string{"read:user", "copilot"},
			RedirectURI: "http://localhost:9876/callback",
			PKCE:        false,
		}
	}

	return p
}

// ProviderID returns the provider identifier.
func (p *OAuthProvider) ProviderID() OAuthProviderID { return p.ID }

// AuthInfo returns the provider's OAuth configuration.
func (p *OAuthProvider) AuthInfo() *OAuthAuthInfo { return p.Info }

// GetAPIKey returns the access token as the API key for this provider.
func (p *OAuthProvider) GetAPIKey(creds *OAuthCredentials) string {
	if creds == nil {
		return ""
	}
	return creds.AccessToken
}

// NewAuthorizationURL builds an authorization URL and returns its PKCE state.
func (p *OAuthProvider) NewAuthorizationURL() (string, *PKCE, error) {
	if p == nil || p.Info == nil {
		return "", nil, fmt.Errorf("OAuth provider is not configured")
	}
	var pkce *PKCE
	params := url.Values{}
	params.Set("client_id", p.Info.ClientID)
	params.Set("redirect_uri", p.Info.RedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", strings.Join(p.Info.Scopes, " "))

	if p.Info.PKCE {
		var err error
		pkce, err = GeneratePKCE()
		if err != nil {
			return "", nil, fmt.Errorf("PKCE generation failed: %w", err)
		}
		params.Set("code_challenge", pkce.Challenge)
		params.Set("code_challenge_method", pkce.ChallengeMethod)
	}

	return p.Info.AuthURL + "?" + params.Encode(), pkce, nil
}

// Login performs the OAuth login flow.
func (p *OAuthProvider) Login(callbacks OAuthLoginCallbacks) (*OAuthCredentials, error) {
	// Build authorization URL
	authURL, pkce, err := p.NewAuthorizationURL()
	if err != nil {
		return nil, err
	}

	// Open URL for user to authorize
	if err := callbacks.OpenURL(authURL); err != nil {
		return nil, fmt.Errorf("failed to open auth URL: %w", err)
	}

	// Poll for credentials
	creds, err := callbacks.Poll()
	if err != nil {
		return nil, fmt.Errorf("OAuth polling failed: %w", err)
	}

	// If we got an auth code, exchange it for tokens
	if creds != nil && creds.AccessToken == "" {
		code := strings.TrimSpace(creds.Code)
		if code == "" {
			return nil, fmt.Errorf("OAuth callback did not return an access token or authorization code")
		}
		tokenCreds, err := p.exchangeCode(code, pkce)
		if err != nil {
			return nil, fmt.Errorf("token exchange failed: %w", err)
		}
		return tokenCreds, nil
	}

	return creds, nil
}

// ExchangeCode exchanges an authorization code for tokens.
func (p *OAuthProvider) ExchangeCode(code string, pkce *PKCE) (*OAuthCredentials, error) {
	return p.exchangeCode(code, pkce)
}

// Refresh refreshes OAuth credentials using a refresh token.
func (p *OAuthProvider) Refresh(creds *OAuthCredentials) (*OAuthCredentials, error) {
	if creds.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", creds.RefreshToken)
	data.Set("client_id", p.Info.ClientID)

	req, err := http.NewRequest("POST", p.Info.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read refresh response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token refresh failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	expiresAt := int64(0)
	if tokenResp.ExpiresIn > 0 {
		expiresAt = time.Now().Unix() + tokenResp.ExpiresIn
	}

	return &OAuthCredentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    expiresAt,
		ProviderID:   string(p.ID),
		Scopes:       tokenResp.Scope,
		TokenType:    tokenResp.TokenType,
	}, nil
}

// exchangeCode exchanges an authorization code for tokens.
func (p *OAuthProvider) exchangeCode(code string, pkce *PKCE) (*OAuthCredentials, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", p.Info.RedirectURI)
	data.Set("client_id", p.Info.ClientID)

	if pkce != nil {
		data.Set("code_verifier", pkce.Verifier)
	}

	req, err := http.NewRequest("POST", p.Info.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	expiresAt := int64(0)
	if tokenResp.ExpiresIn > 0 {
		expiresAt = time.Now().Unix() + tokenResp.ExpiresIn
	}

	return &OAuthCredentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    expiresAt,
		ProviderID:   string(p.ID),
		Scopes:       tokenResp.Scope,
		TokenType:    tokenResp.TokenType,
	}, nil
}

// IsExpired checks if the credentials are expired.
func IsExpired(creds *OAuthCredentials) bool {
	if creds == nil || creds.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() >= creds.ExpiresAt
}

// NeedsRefresh checks if the credentials will expire soon (within 5 minutes).
// This allows preemptive refresh to avoid auth failures mid-session.
func NeedsRefresh(creds *OAuthCredentials) bool {
	if creds == nil || creds.ExpiresAt == 0 {
		return false
	}
	// Refresh if expired or expiring within 5 minutes
	return time.Now().Unix() >= (creds.ExpiresAt - 300)
}

// RefreshIfNeeded checks if credentials need refresh and refreshes them if so.
// Returns the (possibly refreshed) credentials, or an error if refresh failed.
// If no refresh is needed, the original credentials are returned unchanged.
func RefreshIfNeeded(creds *OAuthCredentials, provider OAuthProviderInterface) (*OAuthCredentials, error) {
	if creds == nil || provider == nil {
		return creds, nil
	}
	if !NeedsRefresh(creds) {
		return creds, nil
	}
	if creds.RefreshToken == "" {
		return creds, nil
	}
	newCreds, err := provider.Refresh(creds)
	if err != nil {
		return creds, fmt.Errorf("failed to refresh OAuth token for %s: %w", provider.ProviderID(), err)
	}
	return newCreds, nil
}

// OAuthProviderFactory creates OAuth providers by ID.
func OAuthProviderFactory(id OAuthProviderID) OAuthProviderInterface {
	return NewOAuthProvider(id)
}

// GetOAuthProviders returns all known OAuth providers.
func GetOAuthProviders() []OAuthSelectOption {
	return []OAuthSelectOption{
		{
			ProviderID:  OAuthAnthropic,
			Name:        "Anthropic",
			Description: "Login with Anthropic to use Claude models",
			AuthInfo:    NewOAuthProvider(OAuthAnthropic).Info,
		},
		{
			ProviderID:  OAuthGitHubCopilot,
			Name:        "GitHub Copilot",
			Description: "Login with GitHub to use Copilot models",
			AuthInfo:    NewOAuthProvider(OAuthGitHubCopilot).Info,
		},
		{
			ProviderID:  OAuthOpenAICodex,
			Name:        "OpenAI Codex",
			Description: "Login to use OpenAI Codex models",
			AuthInfo:    NewOAuthProvider(OAuthOpenAICodex).Info,
		},
	}
}
