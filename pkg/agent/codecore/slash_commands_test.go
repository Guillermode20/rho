package codecore

import "testing"

func TestBuiltinSlashCommandsIncludePiCoreCommands(t *testing.T) {
	manager := NewSlashCommandManager()
	want := []string{
		"settings",
		"model",
		"scoped-models",
		"export",
		"import",
		"share",
		"copy",
		"name",
		"session",
		"changelog",
		"hotkeys",
		"fork",
		"clone",
		"tree",
		"login",
		"logout",
		"new",
		"compact",
		"resume",
		"reload",
		"quit",
	}

	for _, name := range want {
		if _, ok := manager.Get(name); !ok {
			t.Fatalf("missing builtin slash command /%s", name)
		}
	}
	if _, ok := manager.Get("commands"); !ok {
		t.Fatal("missing /commands alias for /help")
	}
}
