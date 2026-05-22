package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/earendil-works/rho/pkg/agent"
	"github.com/earendil-works/rho/pkg/agent/export"
	"github.com/earendil-works/rho/pkg/agent/ui"
)

func (im *InteractiveMode) handleSessionsCommand(args []string, title string) {
	im.ui.ClearInput()
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "new":
			im.startNewSession()
			return
		case "list", "switch", "resume":
			im.showSessionSelector(title)
			return
		}
	}
	im.showSessionSelector(title)
}

func (im *InteractiveMode) handleNameCommand(args []string) {
	im.ui.ClearInput()
	if len(args) > 0 {
		im.setSessionName(strings.Join(args, " "))
		return
	}
	current := im.sessionName(im.sessionID)
	im.ui.OpenModalPrompt("Session name", current, func(value string) {
		im.setSessionName(value)
	}, func() {
		im.addSystemMessage("Session naming cancelled.")
	})
}

func (im *InteractiveMode) setSessionName(name string) {
	name = strings.TrimSpace(name)
	names := im.sessionNames()
	if name == "" {
		delete(names, im.sessionID)
		im.addSystemMessage("Session name cleared.")
	} else {
		names[im.sessionID] = name
		im.addSystemMessage(fmt.Sprintf("Session name set to: %s", name))
	}
	im.setUserSetting("sessionNames", names)
	im.ui.SetStatus(im.statusText(""))
}

func (im *InteractiveMode) sessionName(sessionID string) string {
	return im.sessionNames()[sessionID]
}

func (im *InteractiveMode) sessionNames() map[string]string {
	out := make(map[string]string)
	if im.config.Settings == nil {
		return out
	}
	raw := im.config.Settings.Get("sessionNames")
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

func (im *InteractiveMode) showSessionSelector(title string) {
	if im.sessionManager == nil {
		im.addSystemMessage("No session manager is configured.")
		return
	}
	sessions, err := im.sessionManager.List()
	if err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not list sessions: %v", err))
		return
	}
	if len(sessions) == 0 {
		im.addSystemMessage("No saved sessions.")
		return
	}
	items := make([]ui.AutocompleteItem, 0, len(sessions))
	for _, session := range sessions {
		label := session.ID
		if session.ID == im.sessionID {
			label += " (current)"
		}
		desc := strings.TrimSpace(session.Preview)
		if desc == "" {
			desc = fmt.Sprintf("%d messages", session.MessageCount)
		}
		if session.CWD != "" {
			desc = shortenPath(session.CWD) + " - " + desc
		}
		items = append(items, ui.AutocompleteItem{
			Value:       session.ID,
			Label:       label,
			Description: desc,
		})
	}
	im.ui.OpenModalSelector(title, items, func(item ui.AutocompleteItem) {
		im.resumeSession(item.Value)
	}, func() {
		im.addSystemMessage("Session selection cancelled.")
	})
}

func (im *InteractiveMode) resumeSession(sessionID string) {
	header, messages, err := im.sessionManager.Load(sessionID)
	if err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not resume session %s: %v", sessionID, err))
		return
	}
	im.sessionID = sessionID
	if header.CWD != "" {
		im.config.CWD = header.CWD
	}
	im.ui.ClearMessages()
	im.addWelcomeMessage()
	for _, msg := range messages {
		im.ui.AddMessage(msg)
	}
	im.ui.SetStatus(im.statusText(""))
	im.addSystemMessage(fmt.Sprintf("Resumed session %s.", sessionID))
}

func (im *InteractiveMode) startNewSession() {
	im.sessionID = agent.CurrentSessionID()
	im.ui.ClearMessages()
	im.ui.ClearInput()
	im.addWelcomeMessage()
	im.ui.SetStatus(im.statusText(""))
	im.addSystemMessage(fmt.Sprintf("Started new session %s.", im.sessionID))
}

func (im *InteractiveMode) forkSession() {
	if im.sessionManager == nil {
		im.addSystemMessage("No session manager is configured.")
		return
	}
	parentID := im.sessionID
	newID := agent.CurrentSessionID()
	messages := conversationMessages(im.ui.Snapshot())
	header := agent.SessionHeader{
		ID:            newID,
		Timestamp:     time.Now().Format(time.RFC3339),
		CWD:           im.config.CWD,
		ParentSession: parentID,
	}
	if err := im.sessionManager.Save(newID, header, messages); err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not fork session: %v", err))
		return
	}
	im.sessionID = newID
	im.ui.SetStatus(im.statusText(""))
	im.addSystemMessage(fmt.Sprintf("Forked session %s from %s.", newID, parentID))
}

func (im *InteractiveMode) cloneSession() {
	if im.sessionManager == nil {
		im.addSystemMessage("No session manager is configured.")
		return
	}
	newID := agent.CurrentSessionID()
	messages := conversationMessages(im.ui.Snapshot())
	if len(messages) == 0 {
		im.addSystemMessage("No messages to clone.")
		return
	}
	header := agent.SessionHeader{
		ID:        newID,
		Timestamp: time.Now().Format(time.RFC3339),
		CWD:       im.config.CWD,
	}
	if err := im.sessionManager.Save(newID, header, messages); err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not clone session: %v", err))
		return
	}
	im.addSystemMessage(fmt.Sprintf("Cloned session %s (copied %d messages).", newID, len(messages)))
}

func (im *InteractiveMode) shareSession(args []string) {
	im.addSystemMessage("📤 Session sharing...")
	if im.sessionManager == nil {
		im.addSystemMessage("No session manager is configured.")
		return
	}

	// Save current session first
	sessionMessages := conversationMessages(im.ui.Snapshot())
	if len(sessionMessages) == 0 {
		im.addSystemMessage("No messages in session to share.")
		return
	}
	header := agent.SessionHeader{
		ID:        im.sessionID,
		Timestamp: time.Now().Format(time.RFC3339),
		CWD:       im.config.CWD,
	}
	if err := im.sessionManager.Save(im.sessionID, header, sessionMessages); err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not save session for sharing: %v", err))
		return
	}

	// Export to JSON and display share instructions
	sessions, err := im.sessionManager.List()
	if err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not list sessions: %v", err))
		return
	}

	var shareInfo strings.Builder
	shareInfo.WriteString("📤 Session Share\n\n")
	shareInfo.WriteString(fmt.Sprintf("Session: %s\n", im.sessionID))
	shareInfo.WriteString(fmt.Sprintf("Messages: %d\n", len(sessionMessages)))
	shareInfo.WriteString(fmt.Sprintf("Directory: %s\n\n", im.config.CWD))
	shareInfo.WriteString("To share this session:\n")
	shareInfo.WriteString(fmt.Sprintf("  1. Find the session file at:\n     ~/.rho/sessions/%s.json\n", im.sessionID))
	shareInfo.WriteString("  2. Share the file or its contents.\n")
	shareInfo.WriteString("\nAll sessions:\n")
	for _, s := range sessions {
		name := s.ID
		if s.Name != "" {
			name = s.Name
		}
		shareInfo.WriteString(fmt.Sprintf("  - %s (%d msgs, %s)\n", name, s.MessageCount, s.Timestamp[:10]))
	}

	im.addSystemMessage(shareInfo.String())
}

func (im *InteractiveMode) importSession(args []string) {
	if len(args) == 0 {
		im.ui.OpenModalPrompt("Import session", "Path to session JSON file...",
			func(path string) {
				im.doImportSession(path)
			},
			func() {
				im.addSystemMessage("Session import cancelled.")
			},
		)
		return
	}
	im.doImportSession(args[0])
}

func (im *InteractiveMode) doImportSession(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not read file: %v", err))
		return
	}

	var entries []agent.SessionEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not parse session file: %v\nExpected JSONL or JSON array format.", err))
		return
	}

	if len(entries) == 0 {
		im.addSystemMessage("No entries found in session file.")
		return
	}

	// Find header and messages
	var header agent.SessionHeader
	var messages []agent.AgentMessage
	for _, e := range entries {
		if e.Type == "session" {
			header = agent.SessionHeader{
				ID:            im.sessionID,
				Timestamp:     e.Timestamp,
				CWD:           e.CWD,
				ParentSession: e.ParentSession,
			}
		} else if e.Type == "message" && e.Message != nil {
			messages = append(messages, *e.Message)
		}
	}

	if header.ID == "" {
		header = agent.SessionHeader{
			ID:        im.sessionID,
			Timestamp: time.Now().Format(time.RFC3339),
			CWD:       im.config.CWD,
		}
	}

	// Save to our session store
	if err := im.sessionManager.Save(im.sessionID, header, messages); err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not save imported session: %v", err))
		return
	}

	im.ui.ClearMessages()
	for _, msg := range messages {
		im.ui.AddMessage(msg)
	}
	im.ui.SetStatus(im.statusText(""))
	im.addSystemMessage(fmt.Sprintf("✅ Imported %d messages from %s into session %s.", len(messages), path, im.sessionID))
}

func (im *InteractiveMode) exportSession(args []string) {
	if im.sessionManager == nil {
		im.addSystemMessage("No session manager is configured.")
		return
	}

	sessionMessages := conversationMessages(im.ui.Snapshot())
	if len(sessionMessages) == 0 {
		im.addSystemMessage("No messages to export.")
		return
	}

	header := agent.SessionHeader{
		ID:        im.sessionID,
		Timestamp: time.Now().Format(time.RFC3339),
		CWD:       im.config.CWD,
	}
	if err := im.sessionManager.Save(im.sessionID, header, sessionMessages); err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not save session: %v", err))
		return
	}

	filename := fmt.Sprintf("rho-session-%s.html", im.sessionID[:min(20, len(im.sessionID))])
	if len(args) > 0 {
		filename = args[0]
	}

	opts := export.DefaultExportOptions()
	opts.Title = fmt.Sprintf("rho Session %s", im.sessionID[:min(8, len(im.sessionID))])

	if err := export.ExportToHTML(sessionMessages, filename, opts); err != nil {
		im.addSystemMessage(fmt.Sprintf("Could not write HTML export file: %v", err))
		return
	}

	im.addSystemMessage(fmt.Sprintf("✅ Session exported to %s (%d messages).", filename, len(sessionMessages)))
}
