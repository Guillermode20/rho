package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/earendil-works/rho/pkg/agent/extensions"
	"github.com/earendil-works/rho/pkg/agent/ui"
)

type installedExtension struct {
	Name        string
	Value       string
	Version     string
	Description string
	Enabled     bool
	Dir         string
}

func (im *InteractiveMode) showSettingsSelector() {
	apiKeyStatus := "not configured"
	if strings.TrimSpace(im.config.APIKey) != "" {
		apiKeyStatus = "configured"
	}
	showImages := im.settingBool("showImages", true)
	thinkingLevel := im.settingString("thinkingLevel", "off")
	activeTheme := im.activeThemeName()
	items := []ui.AutocompleteItem{
		{Value: "model", Label: "Model", Description: fmt.Sprintf("%s/%s", im.config.Provider, im.config.Model.Name)},
		{Value: "auth", Label: "Authentication", Description: fmt.Sprintf("%s API key %s", im.config.Provider, apiKeyStatus)},
		{Value: "theme", Label: "Theme", Description: activeTheme},
		{Value: "extensions", Label: "Extensions", Description: "Manage installed extensions"},
		{Value: "showImages", Label: "Show images", Description: boolLabel(showImages)},
		{Value: "thinkingLevel", Label: "Thinking level", Description: thinkingLevel},
		{Value: "cwd", Label: "Working directory", Description: im.config.CWD},
		{Value: "commands", Label: "Slash commands", Description: "Show all registered commands"},
	}
	im.ui.OpenModalSelector("Settings", items, func(item ui.AutocompleteItem) {
		switch item.Value {
		case "model":
			im.showModelSelector("")
		case "auth":
			im.showLoginAuthTypeSelector()
		case "theme":
			im.showThemeSelector()
		case "extensions":
			im.showExtensionsSelector()
		case "commands":
			im.showCommandList()
		case "showImages":
			next := !showImages
			im.setUserSetting("showImages", next)
			im.addSystemMessage(fmt.Sprintf("Show images: %s", boolLabel(next)))
			im.showSettingsSelector()
		case "thinkingLevel":
			next := nextThinkingLevel(thinkingLevel)
			im.setUserSetting("thinkingLevel", next)
			im.addSystemMessage(fmt.Sprintf("Thinking level: %s", next))
			im.showSettingsSelector()
		default:
			im.addSystemMessage(fmt.Sprintf("%s: %s", item.Label, item.Description))
		}
	}, func() {
		im.addSystemMessage("Settings closed.")
	})
}

func (im *InteractiveMode) showThemeSelector() {
	if im.config.ThemeManager == nil {
		im.addSystemMessage("No theme manager is configured.")
		return
	}
	names := im.config.ThemeManager.ListThemes()
	if len(names) == 0 {
		im.addSystemMessage("No themes are available.")
		return
	}
	active := im.activeThemeName()
	items := make([]ui.AutocompleteItem, 0, len(names))
	for _, name := range names {
		item := ui.AutocompleteItem{
			Value:       name,
			Label:       name,
			Description: "theme",
		}
		if t, ok := im.config.ThemeManager.GetTheme(name); ok {
			if t.Description != "" {
				item.Description = t.Description
			}
			if t.Dark {
				if item.Description == "" || item.Description == "theme" {
					item.Description = "dark"
				} else {
					item.Description += " | dark"
				}
			}
		}
		if name == active {
			if item.Description == "" || item.Description == "theme" {
				item.Description = "current"
			} else {
				item.Description += " | current"
			}
		}
		items = append(items, item)
	}
	im.ui.OpenModalSelector("Select theme", items, func(item ui.AutocompleteItem) {
		im.selectTheme(item.Value)
	}, func() {
		im.addSystemMessage("Theme selection cancelled.")
	})
}

func (im *InteractiveMode) selectTheme(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		im.showThemeSelector()
		return
	}
	if im.config.ThemeManager == nil {
		im.addSystemMessage("No theme manager is configured.")
		return
	}
	if err := im.config.ThemeManager.SetActive(name); err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not select theme %s: %v", name, err))
		return
	}
	if im.config.Settings != nil {
		im.setUserSetting("theme", name)
	}
	im.applyActiveTheme()
	im.ui.SetStatus(im.statusText(""))
	im.addSystemMessage(fmt.Sprintf("Selected theme: %s", name))
}

func (im *InteractiveMode) applyActiveTheme() {
	if im.ui == nil || im.config.ThemeManager == nil {
		return
	}
	im.ui.ApplyTheme(im.config.ThemeManager.Active())
}

func (im *InteractiveMode) getInstalledExtensions() []installedExtension {
	var result []installedExtension
	seen := make(map[string]bool)

	for _, dir := range im.config.ExtDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			name := entry.Name()
			if seen[name] {
				continue
			}
			seen[name] = true

			extDir := filepath.Join(dir, name)
			enabled := true
			if _, err := os.Stat(filepath.Join(extDir, ".disabled")); err == nil {
				enabled = false
			}

			// Read version and description from manifest
			version := "0.1.0"
			description := ""

			// 1. rho.toml
			if data, err := os.ReadFile(filepath.Join(extDir, "rho.toml")); err == nil {
				manifest, err := extensions.ParseTOML(string(data))
				if err == nil {
					version = manifest.Version
					description = manifest.Description
					if manifest.Name != "" {
						name = manifest.Name
					}
				}
			} else if data, err := os.ReadFile(filepath.Join(extDir, "extension.json")); err == nil {
				var m struct {
					Name    string `json:"name"`
					Version string `json:"version"`
					Desc    string `json:"description"`
				}
				if json.Unmarshal(data, &m) == nil {
					version = m.Version
					description = m.Desc
					if m.Name != "" {
						name = m.Name
					}
				}
			}

			result = append(result, installedExtension{
				Name:        name,
				Value:       entry.Name(), // Store the original directory name as Value
				Version:     version,
				Description: description,
				Enabled:     enabled,
				Dir:         extDir,
			})
		}
	}
	return result
}

func (im *InteractiveMode) showExtensionsSelector() {
	installed := im.getInstalledExtensions()

	items := make([]ui.AutocompleteItem, 0, len(installed))
	for _, ext := range installed {
		status := "ON"
		if !ext.Enabled {
			status = "OFF"
		}

		desc := fmt.Sprintf("[%s] v%s — %s", status, ext.Version, ext.Description)
		items = append(items, ui.AutocompleteItem{
			Value:       ext.Name,
			Label:       ext.Name,
			Description: desc,
		})
	}

	im.ui.OpenModalSelector("Manage Extensions", items, func(item ui.AutocompleteItem) {
		var selected *installedExtension
		for i := range installed {
			if installed[i].Name == item.Value {
				selected = &installed[i]
				break
			}
		}
		if selected != nil {
			im.showExtensionActions(*selected)
		}
	}, func() {
		im.addSystemMessage("Extensions closed.")
	})
}

func (im *InteractiveMode) showExtensionActions(ext installedExtension) {
	toggleLabel := "Enable"
	if ext.Enabled {
		toggleLabel = "Disable"
	}

	items := []ui.AutocompleteItem{
		{Value: "toggle", Label: toggleLabel, Description: "Toggle enabled/disabled status"},
		{Value: "reload", Label: "Reload", Description: "Hot reload this extension"},
		{Value: "uninstall", Label: "Uninstall", Description: "Remove extension files and uninstall"},
		{Value: "back", Label: "Back", Description: "Return to extensions list"},
	}

	im.ui.OpenModalSelector(fmt.Sprintf("Extension: %s", ext.Name), items, func(item ui.AutocompleteItem) {
		switch item.Value {
		case "toggle":
			disabledPath := filepath.Join(ext.Dir, ".disabled")
			if ext.Enabled {
				err := os.WriteFile(disabledPath, []byte("disabled"), 0644)
				if err != nil {
					im.addSystemMessage(fmt.Sprintf("Failed to disable extension: %v", err))
				} else {
					im.addSystemMessage(fmt.Sprintf("Disabled extension: %s. Restart session to apply.", ext.Name))
				}
			} else {
				err := os.Remove(disabledPath)
				if err != nil && !os.IsNotExist(err) {
					im.addSystemMessage(fmt.Sprintf("Failed to enable extension: %v", err))
				} else {
					im.addSystemMessage(fmt.Sprintf("Enabled extension: %s. Restart session to apply.", ext.Name))
				}
			}
			im.showExtensionsSelector()
		case "reload":
			if ext.Enabled {
				err := im.extRuntime.ReloadExtensionFromDir(ext.Name, ext.Dir, im.extensionUIContext())
				if err != nil {
					im.addSystemMessage(fmt.Sprintf("Failed to reload %s: %v", ext.Name, err))
				} else {
					im.addSystemMessage(fmt.Sprintf("Reloaded extension: %s", ext.Name))
				}
			} else {
				im.addSystemMessage(fmt.Sprintf("Cannot reload disabled extension: %s", ext.Name))
			}
			im.showExtensionsSelector()
		case "uninstall":
			im.ui.OpenModalConfirm("Uninstall Extension", fmt.Sprintf("Are you sure you want to uninstall %s?", ext.Name), func() {
				err := os.RemoveAll(ext.Dir)
				if err != nil {
					im.addSystemMessage(fmt.Sprintf("Failed to uninstall %s: %v", ext.Name, err))
				} else {
					im.extRuntime.Unregister(ext.Name)
					im.addSystemMessage(fmt.Sprintf("Uninstalled extension: %s", ext.Name))
				}
				im.showExtensionsSelector()
			}, func() {
				im.showExtensionActions(ext)
			})
		case "back":
			im.showExtensionsSelector()
		}
	}, func() {
		im.showExtensionsSelector()
	})
}

func (im *InteractiveMode) activeThemeName() string {
	if im.config.ThemeManager == nil {
		return "default"
	}
	return im.config.ThemeManager.ActiveName()
}

func (im *InteractiveMode) settingBool(key string, fallback bool) bool {
	if im.config.Settings == nil {
		return fallback
	}
	value := im.config.Settings.Get(key)
	if value == nil {
		return fallback
	}
	if b, ok := value.(bool); ok {
		return b
	}
	return fallback
}

func (im *InteractiveMode) settingString(key, fallback string) string {
	if im.config.Settings == nil {
		return fallback
	}
	if s := im.config.Settings.GetString(key); s != "" {
		return s
	}
	return fallback
}

func (im *InteractiveMode) setUserSetting(key string, value interface{}) {
	if im.config.Settings == nil {
		im.addSystemMessage("No settings manager is configured.")
		return
	}
	if err := im.config.Settings.SetUser(key, value); err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not save setting %s: %v", key, err))
	}
}

func boolLabel(value bool) string {
	if value {
		return "on"
	}
	return "off"
}

func nextThinkingLevel(current string) string {
	levels := []string{"off", "low", "medium", "high"}
	for i, level := range levels {
		if current == level {
			return levels[(i+1)%len(levels)]
		}
	}
	return "off"
}

func shortenPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

func (im *InteractiveMode) handleReloadCommand() {
	im.ui.ClearInput()
	im.addSystemMessage("Reloading extensions, skills, prompts, and themes...")

	// Track results
	var summary []string

	// 1. Reload themes
	if im.config.ThemeManager != nil {
		if err := im.config.ThemeManager.LoadThemes(); err != nil {
			summary = append(summary, fmt.Sprintf("Themes: error — %v", err))
		} else {
			// Re-apply the active theme
			active := im.config.ThemeManager.ActiveName()
			if active != "" {
				_ = im.config.ThemeManager.SetActive(active)
			}
			im.applyActiveTheme()
			summary = append(summary, fmt.Sprintf("Themes: reloaded (%d available)", len(im.config.ThemeManager.ListThemes())))
		}
	} else {
		summary = append(summary, "Themes: skipped (no theme manager)")
	}

	// 2. Reload extensions — collect names first
	oldExts := im.extRuntime.GetAllExtensions()
	extNames := make([]string, 0, len(oldExts))
	for _, ext := range oldExts {
		extNames = append(extNames, ext.Name)
	}

	// Stop all processes and watchers
	im.extRuntime.StopAllProcesses()

	// Unregister all extensions
	for _, name := range extNames {
		im.extRuntime.Unregister(name)
	}

	// Reload extensions from configured directories
	result := extensions.LoadExtensions(im.config.ExtDirs, im.extRuntime)

	loadedCount := len(result.Loaded)
	if loadedCount > 0 {
		summary = append(summary, fmt.Sprintf("Extensions: %d loaded", loadedCount))
	}
	if len(result.Skipped) > 0 {
		summary = append(summary, fmt.Sprintf("Extensions: %d skipped", len(result.Skipped)))
	}
	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			summary = append(summary, fmt.Sprintf("Extension error: %s", err))
		}
	}
	if loadedCount == 0 && len(result.Errors) == 0 {
		summary = append(summary, "Extensions: none found")
	}

	// 3. Update extension statuses on the UI
	im.ui.SetStatus(im.statusText(""))

	// Show summary
	for _, line := range summary {
		im.addSystemMessage(line)
	}
	im.addSystemMessage("Reload complete.")
}
