package auth

import (
	"os"
	"testing"
)

func tempFile(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("", "auth-test-*.json")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func TestNewAuthStorage(t *testing.T) {
	s := NewAuthStorage(tempFile(t))
	if s == nil {
		t.Fatal("expected non-nil")
	}
}

func TestSetGetAPIKey(t *testing.T) {
	f := tempFile(t)
	defer os.Remove(f)
	s := NewAuthStorage(f)
	s.SetAPIKey("anthropic", "sk-ant-key")
	key, ok := s.GetAPIKey("anthropic")
	if !ok {
		t.Fatal("expected ok")
	}
	if key != "sk-ant-key" {
		t.Errorf("expected 'sk-ant-key', got '%s'", key)
	}

	// Verify persistence
	s2 := NewAuthStorage(f)
	key2, ok2 := s2.GetAPIKey("anthropic")
	if !ok2 {
		t.Fatal("expected ok after reload")
	}
	if key2 != "sk-ant-key" {
		t.Errorf("expected 'sk-ant-key', got '%s'", key2)
	}
}

func TestGetAPIKeyNotFound(t *testing.T) {
	f := tempFile(t)
	defer os.Remove(f)
	s := NewAuthStorage(f)
	_, ok := s.GetAPIKey("nonexistent")
	if ok {
		t.Error("expected not ok")
	}
}

func TestHasAPIKey(t *testing.T) {
	f := tempFile(t)
	defer os.Remove(f)
	s := NewAuthStorage(f)
	if s.HasAPIKey("test") {
		t.Error("expected false initially")
	}
	s.SetAPIKey("test", "val")
	if !s.HasAPIKey("test") {
		t.Error("expected true after set")
	}
}

func TestDeleteAPIKey(t *testing.T) {
	f := tempFile(t)
	defer os.Remove(f)
	s := NewAuthStorage(f)
	s.SetAPIKey("openai", "sk-test")
	s.DeleteAPIKey("openai")
	if s.HasAPIKey("openai") {
		t.Error("expected deleted")
	}
}

func TestGetAllProviders(t *testing.T) {
	f := tempFile(t)
	defer os.Remove(f)
	s := NewAuthStorage(f)
	s.SetAPIKey("a", "1")
	s.SetAPIKey("b", "2")
	s.SetAPIKey("c", "3")
	providers := s.GetAllProviders()
	if len(providers) != 3 {
		t.Errorf("expected 3 providers, got %d", len(providers))
	}
}

func TestClearKeys(t *testing.T) {
	f := tempFile(t)
	defer os.Remove(f)
	s := NewAuthStorage(f)
	s.SetAPIKey("key1", "val1")
	s.SetAPIKey("key2", "val2")
	s.Clear()
	if s.HasAPIKey("key1") {
		t.Error("expected cleared")
	}
	if s.HasAPIKey("key2") {
		t.Error("expected cleared")
	}
	if len(s.GetAllProviders()) != 0 {
		t.Error("expected empty")
	}
}

func TestOAuthStore(t *testing.T) {
	f := tempFile(t)
	defer os.Remove(f)
	store := NewOAuthStore(f)

	// Initially empty
	_, ok := store.Get("google")
	if ok {
		t.Fatal("expected no credentials initially")
	}

	// Save
	cred := &OAuthCredential{
		Provider:     "google",
		AccessToken:  "access-token-123",
		RefreshToken: "refresh-token-456",
		ExpiresAt:    1234567890,
	}
	if err := store.Save(cred); err != nil {
		t.Fatal(err)
	}

	// Retrieve
	got, ok := store.Get("google")
	if !ok {
		t.Fatal("expected ok after save")
	}
	if got.AccessToken != "access-token-123" {
		t.Errorf("expected 'access-token-123', got '%s'", got.AccessToken)
	}
	if got.RefreshToken != "refresh-token-456" {
		t.Errorf("expected 'refresh-token-456', got '%s'", got.RefreshToken)
	}
	if got.ExpiresAt != 1234567890 {
		t.Errorf("expected 1234567890, got %d", got.ExpiresAt)
	}

	// Persistence
	store2 := NewOAuthStore(f)
	got2, ok2 := store2.Get("google")
	if !ok2 {
		t.Fatal("expected persistence")
	}
	if got2.AccessToken != "access-token-123" {
		t.Errorf("expected 'access-token-123', got '%s'", got2.AccessToken)
	}
}

func TestOAuthDelete(t *testing.T) {
	f := tempFile(t)
	defer os.Remove(f)
	store := NewOAuthStore(f)
	store.Save(&OAuthCredential{
		Provider:    "google",
		AccessToken: "token",
	})
	store.Delete("google")
	_, ok := store.Get("google")
	if ok {
		t.Error("expected deleted")
	}
}

func TestLoadCorruptedFile(t *testing.T) {
	f := tempFile(t)
	defer os.Remove(f)
	os.WriteFile(f, []byte("{invalid"), 0644)

	s := NewAuthStorage(f)
	if s.HasAPIKey("any") {
		t.Error("expected no keys for corrupted file")
	}

	store := NewOAuthStore(f)
	_, ok := store.Get("any")
	if ok {
		t.Error("expected no creds for corrupted file")
	}
}
