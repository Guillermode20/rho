// Package auth manages API key storage and OAuth credentials for AI providers.
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// AuthStorage manages API keys and OAuth credentials.
type AuthStorage struct {
	mu       sync.RWMutex
	filePath string
	keys     map[string]string
}

// NewAuthStorage creates an auth storage backed by a JSON file.
func NewAuthStorage(filePath string) *AuthStorage {
	s := &AuthStorage{
		filePath: filePath,
		keys:     make(map[string]string),
	}
	s.load()
	return s
}

// SetAPIKey stores an API key for a provider.
func (s *AuthStorage) SetAPIKey(provider, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[provider] = key
	return s.save()
}

// GetAPIKey retrieves an API key for a provider.
func (s *AuthStorage) GetAPIKey(provider string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key, ok := s.keys[provider]
	return key, ok
}

// DeleteAPIKey removes an API key.
func (s *AuthStorage) DeleteAPIKey(provider string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.keys, provider)
	return s.save()
}

// HasAPIKey checks if a provider has a stored key.
func (s *AuthStorage) HasAPIKey(provider string) bool {
	_, ok := s.GetAPIKey(provider)
	return ok
}

// GetAllProviders returns all providers with stored keys.
func (s *AuthStorage) GetAllProviders() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var providers []string
	for p := range s.keys {
		providers = append(providers, p)
	}
	return providers
}

// Clear removes all stored keys.
func (s *AuthStorage) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys = make(map[string]string)
	return s.save()
}

func (s *AuthStorage) load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}
	var keys map[string]string
	if err := json.Unmarshal(data, &keys); err != nil {
		return
	}
	s.keys = keys
}

func (s *AuthStorage) save() error {
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cannot create auth dir: %w", err)
	}
	data, err := json.MarshalIndent(s.keys, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal keys: %w", err)
	}
	if err := os.WriteFile(s.filePath, data, 0600); err != nil {
		return fmt.Errorf("cannot write auth file: %w", err)
	}
	return nil
}

// OAuthCredential represents stored OAuth credentials.
type OAuthCredential struct {
	Provider     string `json:"provider"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresAt    int64  `json:"expiresAt,omitempty"`
}

// OAuthStore manages OAuth credentials.
type OAuthStore struct {
	mu       sync.RWMutex
	filePath string
	creds    map[string]*OAuthCredential
}

// NewOAuthStore creates an OAuth credential store.
func NewOAuthStore(filePath string) *OAuthStore {
	s := &OAuthStore{
		filePath: filePath,
		creds:    make(map[string]*OAuthCredential),
	}
	s.load()
	return s
}

// Save stores OAuth credentials.
func (s *OAuthStore) Save(cred *OAuthCredential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.creds[cred.Provider] = cred
	return s.save()
}

// Get retrieves OAuth credentials for a provider.
func (s *OAuthStore) Get(provider string) (*OAuthCredential, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cred, ok := s.creds[provider]
	return cred, ok
}

// Delete removes OAuth credentials.
func (s *OAuthStore) Delete(provider string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.creds, provider)
	return s.save()
}

func (s *OAuthStore) load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}
	var creds map[string]*OAuthCredential
	if err := json.Unmarshal(data, &creds); err != nil {
		return
	}
	s.creds = creds
}

func (s *OAuthStore) save() error {
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0600)
}
