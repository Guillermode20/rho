package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/auth"
	"github.com/earendil-works/rho/pkg/agent/codecore"
	"github.com/earendil-works/rho/pkg/agent/extensions"
	agenttheme "github.com/earendil-works/rho/pkg/agent/theme"
	"github.com/earendil-works/rho/pkg/ai"
	"github.com/earendil-works/rho/pkg/tui"
)

func TestChatModelSubmitClearsInputThroughCallback(t *testing.T) {
	model := newChatModel("rho")
	var submitted string
	model.onSubmit = func(value string) {
		submitted = value
		model.ClearInput()
	}

	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if submitted != "hello" {
		t.Fatalf("submitted %q, want hello", submitted)
	}
	if got := string(model.input); got != "" {
		t.Fatalf("input after submit = %q, want empty", got)
	}
}

func TestChatModelRenderShowsPlaceholderWhenInputEmpty(t *testing.T) {
	model := newChatModel("rho")
	model.width = 60
	model.height = 12

	view := tui.StripANSI(model.View())
	if !strings.Contains(view, "Type your message...") {
		t.Fatalf("view did not contain placeholder:\n%s", view)
	}
	if !strings.Contains(view, "rho is ready") {
		t.Fatalf("view did not contain empty state:\n%s", view)
	}
	if !strings.Contains(view, "PgUp/PgDn scroll") {
		t.Fatalf("view did not contain key hints:\n%s", view)
	}
}

func TestChatModelSlashAutocompleteAppliesSelection(t *testing.T) {
	model := newChatModel("rho")
	model.onAutocomplete = func(text string, cursor int) []autocompleteItem {
		if text == "/" && cursor == 1 {
			return []autocompleteItem{
				{Value: "/help", Label: "/help", Description: "Show help"},
				{Value: "/login", Label: "/login", Description: "Configure auth"},
			}
		}
		return nil
	}

	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if len(model.autocomplete) != 2 {
		t.Fatalf("autocomplete count = %d, want 2", len(model.autocomplete))
	}

	model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model.Update(tea.KeyMsg{Type: tea.KeyTab})

	if got := string(model.input); got != "/login " {
		t.Fatalf("input = %q, want /login plus trailing space", got)
	}
	if len(model.autocomplete) != 0 {
		t.Fatalf("autocomplete stayed open after apply")
	}
}

func TestInteractiveAutocompleteIncludesCommandsModelsAndProviders(t *testing.T) {
	store := auth.NewAuthStorage(filepath.Join(t.TempDir(), "keys.json"))
	im := NewInteractiveMode(&RuntimeConfig{
		Model:       ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider:    ai.ProviderAnthropic,
		CWD:         t.TempDir(),
		AuthStorage: store,
	})

	commandItems := im.autocomplete("/lo", len("/lo"))
	if !autocompleteHasValue(commandItems, "/login") || !autocompleteHasValue(commandItems, "/logout") {
		t.Fatalf("command autocomplete missing login/logout: %#v", commandItems)
	}

	modelItems := im.autocomplete("/model gpt", len("/model gpt"))
	if !autocompleteHasValue(modelItems, "/model gpt-4o") {
		t.Fatalf("model autocomplete missing gpt-4o: %#v", modelItems)
	}

	providerItems := im.autocomplete("/login anth", len("/login anth"))
	if !autocompleteHasValue(providerItems, "/login anthropic") {
		t.Fatalf("provider autocomplete missing anthropic: %#v", providerItems)
	}
}

func TestInteractiveAutocompleteIncludesExtensionCommands(t *testing.T) {
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
	})
	im.extRuntime.Register(&extensions.ExtensionDef{
		Name: "demo-extension",
		SlashCommands: []extensions.SlashCommand{
			{Name: "demo-cmd", Description: "Run the demo command"},
		},
	})

	items := im.autocomplete("/demo", len("/demo"))
	if !autocompleteHasValue(items, "/demo-cmd") {
		t.Fatalf("extension command autocomplete missing /demo-cmd: %#v", items)
	}
}

func TestInteractiveLoginWithoutArgsOpensAuthTypeSelector(t *testing.T) {
	store := auth.NewAuthStorage(filepath.Join(t.TempDir(), "keys.json"))
	im := NewInteractiveMode(&RuntimeConfig{
		Model:       ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider:    ai.ProviderAnthropic,
		CWD:         t.TempDir(),
		AuthStorage: store,
	})

	im.handleSubmit("/login")

	if im.ui.selectorTitle != "Login method" {
		t.Fatalf("selector title = %q, want Login method", im.ui.selectorTitle)
	}
	if !autocompleteHasValue(im.ui.selectorItems, "api-key") || !autocompleteHasValue(im.ui.selectorItems, "oauth") {
		t.Fatalf("auth type selector missing expected options: %#v", im.ui.selectorItems)
	}
}

func TestInteractiveLoginApiKeyMethodOpensProviderSelector(t *testing.T) {
	store := auth.NewAuthStorage(filepath.Join(t.TempDir(), "keys.json"))
	im := NewInteractiveMode(&RuntimeConfig{
		Model:       ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider:    ai.ProviderAnthropic,
		CWD:         t.TempDir(),
		AuthStorage: store,
	})

	im.handleSubmit("/login")
	im.ui.applySelector()

	if im.ui.selectorTitle != "Login provider" {
		t.Fatalf("selector title = %q, want Login provider", im.ui.selectorTitle)
	}
	if !autocompleteHasValue(im.ui.selectorItems, "anthropic") {
		t.Fatalf("provider selector missing anthropic: %#v", im.ui.selectorItems)
	}
}

func TestInteractiveLoginOAuthMethodOpensOAuthProviderSelector(t *testing.T) {
	store := auth.NewAuthStorage(filepath.Join(t.TempDir(), "keys.json"))
	im := NewInteractiveMode(&RuntimeConfig{
		Model:       ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider:    ai.ProviderAnthropic,
		CWD:         t.TempDir(),
		AuthStorage: store,
	})

	im.handleSubmit("/login")
	im.ui.selectorIdx = 1
	im.ui.applySelector()

	if im.ui.selectorTitle != "OAuth provider" {
		t.Fatalf("selector title = %q, want OAuth provider", im.ui.selectorTitle)
	}
	if !autocompleteHasValue(im.ui.selectorItems, "anthropic") {
		t.Fatalf("oauth selector missing anthropic: %#v", im.ui.selectorItems)
	}
}

func TestChatModelSelectorFiltersAndSelects(t *testing.T) {
	model := newChatModel("rho")
	var selected string
	model.OpenSelector("Select model", []autocompleteItem{
		{Value: "anthropic/claude-sonnet-4-20250514", Label: "claude-sonnet-4-20250514", Description: "anthropic"},
		{Value: "openai/gpt-4o", Label: "gpt-4o", Description: "openai"},
	}, func(item autocompleteItem) {
		selected = item.Value
	}, nil)

	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("gpt")})
	if len(model.selectorItems) != 1 {
		t.Fatalf("filtered selector count = %d, want 1", len(model.selectorItems))
	}
	if model.selectorItems[0].Value != "openai/gpt-4o" {
		t.Fatalf("filtered selector item = %q, want openai/gpt-4o", model.selectorItems[0].Value)
	}

	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if selected != "openai/gpt-4o" {
		t.Fatalf("selected = %q, want openai/gpt-4o", selected)
	}
}

func TestChatModelSelectorKeepsFocusWithNoMatches(t *testing.T) {
	model := newChatModel("rho")
	model.OpenSelector("Select model", []autocompleteItem{
		{Value: "openai/gpt-4o", Label: "gpt-4o", Description: "openai"},
	}, nil, nil)

	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("zzz")})
	if len(model.selectorItems) != 0 {
		t.Fatalf("selectorItems = %d, want 0", len(model.selectorItems))
	}

	model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if len(model.selectorItems) != 1 {
		t.Fatalf("selectorItems after clearing filter = %d, want 1", len(model.selectorItems))
	}
	if got := string(model.input); got != "" {
		t.Fatalf("main input changed while selector was focused: %q", got)
	}
}

func TestInteractiveUISelectRequestUsesSelector(t *testing.T) {
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
	})
	resp := make(chan uiStringResponse, 1)

	im.handleCustomMessage(uiSelectRequestMsg{
		Title:   "Pick one",
		Options: []string{"first", "second"},
		Resp:    resp,
	})
	im.ui.Update(tea.KeyMsg{Type: tea.KeyDown})
	im.ui.Update(tea.KeyMsg{Type: tea.KeyEnter})

	result := <-resp
	if result.Err != nil {
		t.Fatalf("select returned error: %v", result.Err)
	}
	if result.Value != "second" {
		t.Fatalf("select value = %q, want second", result.Value)
	}
}

func TestInteractiveUIInputRequestUsesPrompt(t *testing.T) {
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
	})
	resp := make(chan uiStringResponse, 1)

	im.handleCustomMessage(uiInputRequestMsg{
		Title:       "Name",
		Placeholder: "session name",
		Resp:        resp,
	})
	im.ui.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("demo")})
	im.ui.Update(tea.KeyMsg{Type: tea.KeyEnter})

	result := <-resp
	if result.Err != nil {
		t.Fatalf("input returned error: %v", result.Err)
	}
	if result.Value != "demo" {
		t.Fatalf("input value = %q, want demo", result.Value)
	}
}

func TestInteractiveExtensionStatusUpdatesFooter(t *testing.T) {
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
	})

	im.handleCustomMessage(AgentExtensionStatusMsg{Key: "example", Text: "active"})

	if !strings.Contains(im.ui.status, "active") {
		t.Fatalf("status = %q, want extension status active", im.ui.status)
	}
}

func TestInteractiveModelSelectorChangesCurrentModel(t *testing.T) {
	store := auth.NewAuthStorage(filepath.Join(t.TempDir(), "keys.json"))
	im := NewInteractiveMode(&RuntimeConfig{
		Model:       ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider:    ai.ProviderAnthropic,
		CWD:         t.TempDir(),
		AuthStorage: store,
	})

	im.handleSubmit("/model gpt-4o")

	if im.config.Model.Name != "gpt-4o" {
		t.Fatalf("model = %q, want gpt-4o", im.config.Model.Name)
	}
	if im.config.Provider != ai.ProviderOpenAI {
		t.Fatalf("provider = %q, want openai", im.config.Provider)
	}
}

func TestInteractiveSettingsCommandOpensSelector(t *testing.T) {
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
	})

	im.handleSubmit("/settings")

	if im.ui.selectorTitle != "Settings" {
		t.Fatalf("selector title = %q, want Settings", im.ui.selectorTitle)
	}
	if !autocompleteHasValue(im.ui.selectorItems, "model") {
		t.Fatalf("settings selector missing model item: %#v", im.ui.selectorItems)
	}
}

func TestInteractiveSettingsTogglePersistsShowImages(t *testing.T) {
	settingsDir := t.TempDir()
	settings := codecore.NewSettingsManager(settingsDir, "")
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
		Settings: settings,
	})

	im.handleSubmit("/settings")
	for i, item := range im.ui.selectorItems {
		if item.Value == "showImages" {
			im.ui.selectorIdx = i
			break
		}
	}
	im.ui.applySelector()

	reloaded := codecore.NewSettingsManager(settingsDir, "")
	if got := reloaded.GetBool("showImages"); got {
		t.Fatalf("showImages persisted as %v, want false", got)
	}
}

func TestInteractiveSettingsCyclesThinkingLevel(t *testing.T) {
	settingsDir := t.TempDir()
	settings := codecore.NewSettingsManager(settingsDir, "")
	if err := settings.SetUser("thinkingLevel", "off"); err != nil {
		t.Fatal(err)
	}
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
		Settings: settings,
	})

	im.handleSubmit("/settings")
	for i, item := range im.ui.selectorItems {
		if item.Value == "thinkingLevel" {
			im.ui.selectorIdx = i
			break
		}
	}
	im.ui.applySelector()

	reloaded := codecore.NewSettingsManager(settingsDir, "")
	if got := reloaded.GetString("thinkingLevel"); got != "low" {
		t.Fatalf("thinkingLevel = %q, want low", got)
	}
}

func TestInteractiveAppKeybindingOpensModelSelector(t *testing.T) {
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
	})

	im.ui.Update(tea.KeyMsg{Type: tea.KeyCtrlL})

	if im.ui.selectorTitle != "Select model" {
		t.Fatalf("selector title = %q, want Select model", im.ui.selectorTitle)
	}
}

func TestInteractiveAppKeybindingOpensSettingsSelector(t *testing.T) {
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
	})

	im.ui.Update(tea.KeyMsg{Type: tea.KeyCtrlP})

	if im.ui.selectorTitle != "Settings" {
		t.Fatalf("selector title = %q, want Settings", im.ui.selectorTitle)
	}
}

func TestInteractiveAppKeybindingCyclesThinkingLevel(t *testing.T) {
	settingsDir := t.TempDir()
	settings := codecore.NewSettingsManager(settingsDir, "")
	if err := settings.SetUser("thinkingLevel", "off"); err != nil {
		t.Fatal(err)
	}
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
		Settings: settings,
	})

	im.ui.Update(tea.KeyMsg{Type: tea.KeyCtrlT})

	reloaded := codecore.NewSettingsManager(settingsDir, "")
	if got := reloaded.GetString("thinkingLevel"); got != "low" {
		t.Fatalf("thinkingLevel = %q, want low", got)
	}
}

func TestInteractiveSessionsCommandOpensResumeSelector(t *testing.T) {
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
	})
	im.sessionManager = agent.NewSessionManager(filepath.Join(t.TempDir(), "sessions"))
	mustSaveTestSession(t, im.sessionManager, "session_a", im.config.CWD, "hello from saved session")

	im.handleSubmit("/sessions")

	if im.ui.selectorTitle != "Resume session" {
		t.Fatalf("selector title = %q, want Resume session", im.ui.selectorTitle)
	}
	if !autocompleteHasValue(im.ui.selectorItems, "session_a") {
		t.Fatalf("session selector missing saved session: %#v", im.ui.selectorItems)
	}
}

func TestInteractiveTreeCommandOpensSessionTreeSelector(t *testing.T) {
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
	})
	im.sessionManager = agent.NewSessionManager(filepath.Join(t.TempDir(), "sessions"))
	mustSaveTestSession(t, im.sessionManager, "session_a", im.config.CWD, "tree message")

	im.handleSubmit("/tree")

	if im.ui.selectorTitle != "Session tree" {
		t.Fatalf("selector title = %q, want Session tree", im.ui.selectorTitle)
	}
	if !autocompleteHasValue(im.ui.selectorItems, "session_a") {
		t.Fatalf("tree selector missing saved session: %#v", im.ui.selectorItems)
	}
}

func TestInteractiveResumeSessionLoadsTranscript(t *testing.T) {
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
	})
	im.sessionManager = agent.NewSessionManager(filepath.Join(t.TempDir(), "sessions"))
	mustSaveTestSession(t, im.sessionManager, "session_a", im.config.CWD, "saved transcript")

	im.resumeSession("session_a")

	if im.sessionID != "session_a" {
		t.Fatalf("sessionID = %q, want session_a", im.sessionID)
	}
	found := false
	for _, msg := range im.ui.messages {
		if msg.Content == "saved transcript" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("resumed transcript did not include saved message: %#v", im.ui.messages)
	}
}

func TestInteractiveForkSessionCreatesChildSession(t *testing.T) {
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
	})
	im.sessionManager = agent.NewSessionManager(filepath.Join(t.TempDir(), "sessions"))
	parentID := im.sessionID
	im.ui.AddMessage(agent.AgentMessage{Role: ai.RoleUser, Content: "fork me", Timestamp: time.Now().UnixMilli()})

	im.handleSubmit("/fork")

	if im.sessionID == parentID {
		t.Fatalf("sessionID did not change from parent %q", parentID)
	}
	header, messages, err := im.sessionManager.Load(im.sessionID)
	if err != nil {
		t.Fatalf("load forked session: %v", err)
	}
	if header.ParentSession != parentID {
		t.Fatalf("parent session = %q, want %q", header.ParentSession, parentID)
	}
	if len(messages) != 1 || messages[0].Content != "fork me" {
		t.Fatalf("forked messages = %#v, want one copied message", messages)
	}
}

func TestInteractiveNewSessionResetsTranscript(t *testing.T) {
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
	})
	oldID := im.sessionID
	im.ui.AddMessage(agent.AgentMessage{Role: ai.RoleUser, Content: "old message"})

	im.handleSubmit("/new")

	if im.sessionID == oldID {
		t.Fatalf("sessionID did not change from %q", oldID)
	}
	for _, msg := range im.ui.messages {
		if msg.Content == "old message" {
			t.Fatalf("new session retained old transcript: %#v", im.ui.messages)
		}
	}
}

func TestInteractiveNameCommandPersistsSessionName(t *testing.T) {
	settingsDir := t.TempDir()
	settings := codecore.NewSettingsManager(settingsDir, "")
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
		Settings: settings,
	})
	sessionID := im.sessionID

	im.handleSubmit("/name Demo Session")

	reloaded := codecore.NewSettingsManager(settingsDir, "")
	raw := reloaded.Get("sessionNames")
	names, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("sessionNames type = %T, want map", raw)
	}
	if got := names[sessionID]; got != "Demo Session" {
		t.Fatalf("session name = %q, want Demo Session", got)
	}
	if !strings.Contains(im.ui.status, "Demo Session") {
		t.Fatalf("status = %q, want session name", im.ui.status)
	}
}

func TestInteractiveNameCommandWithoutArgsOpensPrompt(t *testing.T) {
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
	})

	im.handleSubmit("/name")

	if im.ui.promptTitle != "Session name" {
		t.Fatalf("prompt title = %q, want Session name", im.ui.promptTitle)
	}
}

func TestInteractiveThemeCommandWithoutArgsOpensSelector(t *testing.T) {
	im := NewInteractiveMode(&RuntimeConfig{
		Model:        ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider:     ai.ProviderAnthropic,
		CWD:          t.TempDir(),
		ThemeManager: agenttheme.NewThemeManager(filepath.Join(t.TempDir(), "themes")),
	})

	im.handleSubmit("/theme")

	if im.ui.selectorTitle != "Select theme" {
		t.Fatalf("selector title = %q, want Select theme", im.ui.selectorTitle)
	}
	if !autocompleteHasValue(im.ui.selectorItems, "default") || !autocompleteHasValue(im.ui.selectorItems, "dracula") {
		t.Fatalf("theme selector missing built-ins: %#v", im.ui.selectorItems)
	}
}

func TestInteractiveThemeCommandSelectsAndPersistsTheme(t *testing.T) {
	settingsDir := t.TempDir()
	settings := codecore.NewSettingsManager(settingsDir, t.TempDir())
	themeManager := agenttheme.NewThemeManager(filepath.Join(t.TempDir(), "themes"))
	im := NewInteractiveMode(&RuntimeConfig{
		Model:        ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider:     ai.ProviderAnthropic,
		CWD:          t.TempDir(),
		Settings:     settings,
		ThemeManager: themeManager,
	})

	im.handleSubmit("/theme dracula")

	if got := themeManager.ActiveName(); got != "dracula" {
		t.Fatalf("active theme = %q, want dracula", got)
	}
	reloaded := codecore.NewSettingsManager(settingsDir, t.TempDir())
	if got := reloaded.GetString("theme"); got != "dracula" {
		t.Fatalf("persisted theme = %q, want dracula", got)
	}
}

func TestInteractiveCopyCommandCopiesLastAssistantMessage(t *testing.T) {
	var copied string
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
		ClipboardWrite: func(text string) error {
			copied = text
			return nil
		},
	})
	im.ui.AddMessage(agent.AgentMessage{Role: ai.RoleAssistant, Model: "claude", Content: "copy me"})

	im.handleSubmit("/copy")

	if copied != "copy me" {
		t.Fatalf("copied = %q, want copy me", copied)
	}
}

func TestInteractiveCopyCommandIgnoresSystemMessages(t *testing.T) {
	var copied string
	im := NewInteractiveMode(&RuntimeConfig{
		Model:    ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider: ai.ProviderAnthropic,
		CWD:      t.TempDir(),
		ClipboardWrite: func(text string) error {
			copied = text
			return nil
		},
	})
	im.ui.AddMessage(agent.AgentMessage{Role: ai.RoleAssistant, Model: "claude", Content: "real assistant"})
	im.addSystemMessage("system message")

	im.handleSubmit("/copy")

	if copied != "real assistant" {
		t.Fatalf("copied = %q, want real assistant", copied)
	}
}

func TestChatModelAsyncMessagesUpdateInsideTeaModel(t *testing.T) {
	model := newChatModel("rho")
	model.onMessage = func(msg tea.Msg) {
		switch m := msg.(type) {
		case tui.AddMessageMsg:
			model.AddMessage(agent.AgentMessage{
				Role:    ai.Role(m.Role),
				Content: m.Content,
				Model:   m.Model,
			})
		}
	}

	model.Update(tui.AddMessageMsg{
		Role:    string(ai.RoleAssistant),
		Content: "done",
		Model:   "test-model",
	})

	if len(model.messages) != 1 {
		t.Fatalf("message count = %d, want 1", len(model.messages))
	}
	if model.messages[0].Content != "done" {
		t.Fatalf("message content = %q, want done", model.messages[0].Content)
	}
}

func autocompleteHasValue(items []autocompleteItem, value string) bool {
	for _, item := range items {
		if item.Value == value {
			return true
		}
	}
	return false
}

func mustSaveTestSession(t *testing.T, manager *agent.SessionManager, id, cwd, content string) {
	t.Helper()
	err := manager.Save(id, agent.SessionHeader{
		ID:        id,
		Timestamp: time.Now().Format(time.RFC3339),
		CWD:       cwd,
	}, []agent.AgentMessage{
		{Role: ai.RoleUser, Content: content, Timestamp: time.Now().UnixMilli()},
	})
	if err != nil {
		t.Fatalf("save session: %v", err)
	}
}

func TestResolveAPIKeyUsesStoredKey(t *testing.T) {
	store := auth.NewAuthStorage(filepath.Join(t.TempDir(), "keys.json"))
	if err := store.SetAPIKey("anthropic", "stored-key"); err != nil {
		t.Fatal(err)
	}

	key := resolveAPIKey(ai.Model{Provider: ai.ProviderAnthropic}, store)
	if key != "stored-key" {
		t.Fatalf("key = %q, want stored-key", key)
	}
}

func TestInteractiveLoginStoresKeyWithoutTranscriptEcho(t *testing.T) {
	store := auth.NewAuthStorage(filepath.Join(t.TempDir(), "keys.json"))
	im := NewInteractiveMode(&RuntimeConfig{
		Model:       ai.Model{Provider: ai.ProviderAnthropic, Name: "claude-sonnet-4-20250514"},
		Provider:    ai.ProviderAnthropic,
		CWD:         t.TempDir(),
		AuthStorage: store,
	})

	im.handleSubmit("/login anthropic")
	im.handleSubmit("sk-test-secret")

	key, ok := store.GetAPIKey("anthropic")
	if !ok || key != "sk-test-secret" {
		t.Fatalf("stored key = %q, %v; want sk-test-secret, true", key, ok)
	}
	for _, msg := range im.ui.messages {
		if strings.Contains(msg.Content, "sk-test-secret") {
			t.Fatalf("secret was echoed in transcript: %q", msg.Content)
		}
	}
}
