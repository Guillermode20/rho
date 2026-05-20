package ai

import (
	"testing"
	"time"
)

func TestIsExpired(t *testing.T) {
	tests := []struct {
		name     string
		creds    *OAuthCredentials
		expected bool
	}{
		{
			name:     "nil credentials are not expired",
			creds:    nil,
			expected: false,
		},
		{
			name:     "zero expires_at is not expired",
			creds:    &OAuthCredentials{ExpiresAt: 0},
			expected: false,
		},
		{
			name:     "future expiry is not expired",
			creds:    &OAuthCredentials{ExpiresAt: time.Now().Unix() + 3600},
			expected: false,
		},
		{
			name:     "past expiry is expired",
			creds:    &OAuthCredentials{ExpiresAt: time.Now().Unix() - 3600},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsExpired(tt.creds)
			if got != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNeedsRefresh(t *testing.T) {
	tests := []struct {
		name     string
		creds    *OAuthCredentials
		expected bool
	}{
		{
			name:     "nil credentials do not need refresh",
			creds:    nil,
			expected: false,
		},
		{
			name:     "zero expires_at does not need refresh",
			creds:    &OAuthCredentials{ExpiresAt: 0},
			expected: false,
		},
		{
			name:     "far future expiry does not need refresh",
			creds:    &OAuthCredentials{ExpiresAt: time.Now().Unix() + 86400},
			expected: false,
		},
		{
			name:     "expiring within 5 minutes needs refresh",
			creds:    &OAuthCredentials{ExpiresAt: time.Now().Unix() + 120}, // 2 min
			expected: true,
		},
		{
			name:     "already expired needs refresh",
			creds:    &OAuthCredentials{ExpiresAt: time.Now().Unix() - 3600},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedsRefresh(tt.creds)
			if got != tt.expected {
				t.Errorf("NeedsRefresh() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRefreshIfNeeded(t *testing.T) {
	t.Run("nil creds returns nil", func(t *testing.T) {
		got, err := RefreshIfNeeded(nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("no refresh needed returns original", func(t *testing.T) {
		creds := &OAuthCredentials{
			AccessToken:  "tok",
			RefreshToken: "refresh",
			ExpiresAt:    time.Now().Unix() + 86400,
		}
		got, err := RefreshIfNeeded(creds, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != creds {
			t.Fatalf("expected original creds, got different pointer")
		}
	})

	t.Run("needs refresh but no refresh token returns original", func(t *testing.T) {
		creds := &OAuthCredentials{
			AccessToken:  "tok",
			RefreshToken: "",
			ExpiresAt:    time.Now().Unix() - 3600,
		}
		got, err := RefreshIfNeeded(creds, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != creds {
			t.Fatalf("expected original creds when no refresh token")
		}
	})
}

func TestGetAPIKey(t *testing.T) {
	provider := NewOAuthProvider(OAuthAnthropic)

	t.Run("nil creds returns empty", func(t *testing.T) {
		got := provider.GetAPIKey(nil)
		if got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("returns access token", func(t *testing.T) {
		creds := &OAuthCredentials{AccessToken: "test-key"}
		got := provider.GetAPIKey(creds)
		if got != "test-key" {
			t.Fatalf("expected test-key, got %q", got)
		}
	})
}

func TestOAuthProviderFactory(t *testing.T) {
	provider := OAuthProviderFactory(OAuthAnthropic)
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
	if provider.ProviderID() != OAuthAnthropic {
		t.Fatalf("expected anthropic, got %s", provider.ProviderID())
	}
	info := provider.AuthInfo()
	if info == nil {
		t.Fatal("expected non-nil auth info")
	}
	if info.Name != "Anthropic" {
		t.Fatalf("expected Anthropic, got %s", info.Name)
	}
	if !info.PKCE {
		t.Fatal("expected PKCE to be true for Anthropic")
	}
}

func TestGetOAuthProviders(t *testing.T) {
	providers := GetOAuthProviders()
	if len(providers) == 0 {
		t.Fatal("expected at least one provider")
	}

	// Check that Anthropic, GitHub Copilot, and OpenAI Codex are present
	ids := make(map[OAuthProviderID]bool)
	for _, p := range providers {
		ids[p.ProviderID] = true
	}
	if !ids[OAuthAnthropic] {
		t.Fatal("expected Anthropic provider")
	}
	if !ids[OAuthGitHubCopilot] {
		t.Fatal("expected GitHub Copilot provider")
	}
	if !ids[OAuthOpenAICodex] {
		t.Fatal("expected OpenAI Codex provider")
	}
}
