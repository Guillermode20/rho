package settings

import (
	"os"
	"testing"
)

func TestNewSettingsManager(t *testing.T) {
	sm := NewSettingsManager("/tmp/rho-settings-test")
	if sm == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestGetString(t *testing.T) {
	sm := NewSettingsManager("/tmp/rho-settings-test")
	sm.Set("model", "gpt-4", ScopeUser)
	val := sm.GetString("model")
	if val != "gpt-4" {
		t.Errorf("expected 'gpt-4', got '%s'", val)
	}
}

func TestGetInt(t *testing.T) {
	sm := NewSettingsManager("/tmp/rho-settings-test")
	sm.Set("maxTokens", 4096, ScopeUser)
	n := sm.GetInt("maxTokens")
	if n != 4096 {
		t.Errorf("expected 4096, got %d", n)
	}
}

func TestGetBool(t *testing.T) {
	sm := NewSettingsManager("/tmp/rho-settings-test")
	sm.Set("autoSave", true, ScopeUser)
	b := sm.GetBool("autoSave")
	if !b {
		t.Error("expected true")
	}
}

func TestGet_NotFound(t *testing.T) {
	sm := NewSettingsManager("/tmp/rho-settings-test")
	_, ok := sm.Get("nonexistent")
	if ok {
		t.Error("expected false for nonexistent key")
	}
}

func TestSetAndGet(t *testing.T) {
	sm := NewSettingsManager("/tmp/rho-settings-test")
	err := sm.Set("testkey", "testvalue", ScopeUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, ok := sm.Get("testkey")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if val != "testvalue" {
		t.Errorf("expected 'testvalue', got '%v'", val)
	}
}

func TestGetScope(t *testing.T) {
	sm := NewSettingsManager("/tmp/rho-settings-test")
	scope := sm.GetScope("nonexistent")
	if scope != ScopeDefault {
		t.Errorf("expected ScopeDefault, got %v", scope)
	}

	sm.Set("scopedkey", "val", ScopeUser)
	scope = sm.GetScope("scopedkey")
	if scope != ScopeUser {
		t.Errorf("expected ScopeUser, got %v", scope)
	}
}

func TestAll(t *testing.T) {
	sm := NewSettingsManager("/tmp/rho-settings-test")
	all := sm.All()
	if all == nil {
		t.Error("expected non-nil map")
	}
}

func TestDrainErrors(t *testing.T) {
	sm := NewSettingsManager("/tmp/rho-settings-test")
	errs := sm.DrainErrors()
	if errs == nil {
		t.Log("no errors to drain (expected)")
	}
}

func TestShortName(t *testing.T) {
	if ShortName("app.model") != "model" {
		t.Errorf("expected 'model', got '%s'", ShortName("app.model"))
	}
	if ShortName("simple") != "simple" {
		t.Errorf("expected 'simple', got '%s'", ShortName("simple"))
	}
}

func TestEnvOverride(t *testing.T) {
	os.Setenv("RHO_MODEL", "claude-sonnet")
	defer os.Unsetenv("RHO_MODEL")

	sm := NewSettingsManager("/tmp/rho-settings-env")
	sm.Load()

	val := sm.GetString("model")
	if val != "claude-sonnet" {
		t.Errorf("expected 'claude-sonnet' from env, got '%s'", val)
	}
}

func TestLoad(t *testing.T) {
	sm := NewSettingsManager("/tmp/rho-settings-load")
	errs := sm.Load()
	if errs == nil {
		t.Log("no load errors")
	}
}
