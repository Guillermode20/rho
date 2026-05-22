package main

import (
	"fmt"
	"strings"

	"github.com/earendil-works/rho/pkg/agent/ui"
	"github.com/earendil-works/rho/pkg/ai"
)

func (im *InteractiveMode) handleModelCommand(args []string) {
	im.ui.ClearInput()
	if len(args) == 0 {
		im.showModelSelector("")
		return
	}
	query := strings.ToLower(strings.Join(args, " "))
	var bestModel *ai.ModelDefinition
	bestPriority := 9999
	for _, model := range ai.DefaultModels() {
		if strings.ToLower(model.Name) == query || strings.ToLower(string(model.Provider)+"/"+model.Name) == query {
			prio := ai.ProviderPriority(model.Provider)
			if prio < bestPriority {
				m := model
				bestModel = &m
				bestPriority = prio
			}
		}
	}
	if bestModel != nil {
		im.selectModel(*bestModel)
		return
	}
	im.showModelSelector(query)
}

func (im *InteractiveMode) showProviderSelector(title, mode string) {
	items := make([]ui.AutocompleteItem, 0)
	for _, provider := range im.availableProviderNames() {
		items = append(items, ui.AutocompleteItem{
			Value:       provider,
			Label:       provider,
			Description: "provider",
		})
	}
	if len(items) == 0 {
		im.addSystemMessage("No providers are available.")
		return
	}
	im.ui.OpenModalSelector(title, items, func(item ui.AutocompleteItem) {
		switch mode {
		case "login":
			im.handleLoginCommand([]string{item.Value})
		case "logout":
			im.handleLogoutCommand([]string{item.Value})
		}
	}, func() {
		im.addSystemMessage(title + " cancelled.")
	})
}

func (im *InteractiveMode) showModelSelector(query string) {
	query = strings.ToLower(query)
	items := make([]ui.AutocompleteItem, 0)
	noAuthItems := make([]ui.AutocompleteItem, 0)
	for _, model := range ai.DefaultModels() {
		value := string(model.Provider) + "/" + model.Name
		haystack := strings.ToLower(value)
		if query != "" && !strings.Contains(haystack, query) {
			continue
		}
		hasAuth := providerHasAuth(model.Provider, im.config.AuthStorage, im.config.OAuthStore)
		item := ui.AutocompleteItem{
			Value:       value,
			Label:       model.Name,
			Description: string(model.Provider),
		}
		if hasAuth {
			items = append(items, item)
		} else {
			noAuthItems = append(noAuthItems, item)
		}
	}
	// Append unavailable models at the end with a muted indicator
	if len(noAuthItems) > 0 {
		for i := range noAuthItems {
			noAuthItems[i].Description += " (no API key)"
		}
		items = append(items, noAuthItems...)
	}
	if len(items) == 0 {
		im.addSystemMessage("No models matched " + query + ".")
		return
	}
	// Check if any models actually have auth configured
	noAvailable := true
	for _, model := range ai.DefaultModels() {
		value := string(model.Provider) + "/" + model.Name
		haystack := strings.ToLower(value)
		if query != "" && !strings.Contains(haystack, query) {
			continue
		}
		if providerHasAuth(model.Provider, im.config.AuthStorage, im.config.OAuthStore) {
			noAvailable = false
			break
		}
	}

	im.ui.OpenModalSelector("Select model", items, func(item ui.AutocompleteItem) {
		for _, model := range ai.DefaultModels() {
			if item.Value == string(model.Provider)+"/"+model.Name {
				im.selectModel(model)
				return
			}
		}
	}, func() {
		im.addSystemMessage("Model selection cancelled.")
	})

	if noAvailable && query == "" {
		im.addSystemMessage("No API keys configured. Configure one with /login to use that provider's models.")
	}
}

func (im *InteractiveMode) selectModel(model ai.ModelDefinition) {
	im.config.Model = ai.Model{
		API:      model.API,
		Provider: model.Provider,
		Name:     model.Name,
		BaseURL:  model.BaseURL,
	}
	im.config.Provider = model.Provider
	im.config.APIKey = resolveAPIKey(im.config.Model, im.config.AuthStorage)
	im.ui.SetModel(model.Name, string(model.Provider))
	im.ui.SetTokenCount(im.ui.TokenCount, model.ContextWindow)
	im.ui.SetStatus(im.statusText(""))
	im.addSystemMessage(fmt.Sprintf("Selected model: %s/%s", model.Provider, model.Name))
}

func (im *InteractiveMode) availableProviderNames() []string {
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
	return names
}

func (im *InteractiveMode) isKnownProvider(provider string) bool {
	for _, name := range im.availableProviderNames() {
		if name == provider {
			return true
		}
	}
	return false
}

func (im *InteractiveMode) autocomplete(text string, cursor int) []ui.AutocompleteItem {
	if cursor < 0 {
		cursor = 0
	}
	runes := []rune(text)
	if cursor > len(runes) {
		cursor = len(runes)
	}
	prefixText := string(runes[:cursor])
	if !strings.HasPrefix(prefixText, "/") || strings.Contains(prefixText, "\n") {
		return nil
	}

	fields := strings.Fields(prefixText)
	if len(fields) == 0 {
		return im.commandAutocomplete("")
	}

	if len(fields) == 1 && !strings.HasSuffix(prefixText, " ") {
		return im.commandAutocomplete(strings.TrimPrefix(fields[0], "/"))
	}

	cmd := strings.TrimPrefix(fields[0], "/")
	argPrefix := ""
	if len(fields) > 1 {
		argPrefix = fields[len(fields)-1]
	}
	if strings.HasSuffix(prefixText, " ") {
		argPrefix = ""
	}

	switch cmd {
	case "model", "models":
		return im.modelAutocomplete(cmd, argPrefix)
	case "login", "logout":
		return im.providerAutocomplete(cmd, argPrefix)
	default:
		return nil
	}
}

func (im *InteractiveMode) commandAutocomplete(prefix string) []ui.AutocompleteItem {
	prefix = strings.ToLower(prefix)
	var items []ui.AutocompleteItem
	seen := make(map[string]bool)
	if im.slashCommands != nil {
		for _, cmd := range im.slashCommands.List() {
			if cmd.Hidden {
				continue
			}
			if prefix != "" && !strings.Contains(strings.ToLower(cmd.Name), prefix) {
				continue
			}
			seen[cmd.Name] = true
			items = append(items, ui.AutocompleteItem{
				Value:       "/" + cmd.Name,
				Label:       "/" + cmd.Name,
				Description: cmd.Description,
			})
		}
	}
	for _, cmd := range []ui.AutocompleteItem{
		{Value: "/tools", Label: "/tools", Description: "Show available agent tools"},
		{Value: "/commands", Label: "/commands", Description: "Show all registered commands"},
	} {
		name := strings.TrimPrefix(cmd.Label, "/")
		if seen[name] {
			continue
		}
		if prefix != "" && !strings.Contains(strings.ToLower(name), prefix) {
			continue
		}
		seen[name] = true
		items = append(items, cmd)
	}
	for _, cmd := range im.extRuntime.GetSlashCommands() {
		if seen[cmd.Name] {
			continue
		}
		if prefix != "" && !strings.Contains(strings.ToLower(cmd.Name), prefix) {
			continue
		}
		seen[cmd.Name] = true
		items = append(items, ui.AutocompleteItem{
			Value:       "/" + cmd.Name,
			Label:       "/" + cmd.Name,
			Description: cmd.Description,
		})
	}
	return items
}

func (im *InteractiveMode) modelAutocomplete(cmd, prefix string) []ui.AutocompleteItem {
	prefix = strings.ToLower(prefix)
	var items []ui.AutocompleteItem
	for _, model := range ai.DefaultModels() {
		value := "/" + cmd + " " + model.Name
		haystack := strings.ToLower(string(model.Provider) + " " + model.Name)
		if prefix != "" && !strings.Contains(haystack, prefix) {
			continue
		}
		desc := string(model.Provider)
		if !providerHasAuth(model.Provider, im.config.AuthStorage, im.config.OAuthStore) {
			desc += " (no API key)"
		}
		items = append(items, ui.AutocompleteItem{
			Value:       value,
			Label:       model.Name,
			Description: desc,
		})
	}
	return items
}

func (im *InteractiveMode) providerAutocomplete(cmd, prefix string) []ui.AutocompleteItem {
	prefix = strings.ToLower(prefix)
	var items []ui.AutocompleteItem
	for _, provider := range im.availableProviderNames() {
		if prefix != "" && !strings.Contains(strings.ToLower(provider), prefix) {
			continue
		}
		items = append(items, ui.AutocompleteItem{
			Value:       "/" + cmd + " " + provider,
			Label:       provider,
			Description: "provider",
		})
	}
	return items
}
