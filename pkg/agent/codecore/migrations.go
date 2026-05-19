package codecore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Migration struct {
	FromVersion int
	ToVersion   int
	Description string
	Migrate     func(data []byte) ([]byte, error)
}

var migrations []Migration

func RegisterMigration(m Migration) {
	migrations = append(migrations, m)
}

func RunMigrations(sessionsDir string) error {
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if err := migrateFile(filepath.Join(sessionsDir, entry.Name())); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: migration failed for %s: %v\n", entry.Name(), err)
		}
	}
	return nil
}

func migrateFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	version := detectVersion(data)
	migrated := data
	for _, m := range migrations {
		if version >= m.FromVersion && version < m.ToVersion {
			result, err := m.Migrate(migrated)
			if err != nil {
				return fmt.Errorf("migration %d→%d: %w", m.FromVersion, m.ToVersion, err)
			}
			migrated = result
			version = m.ToVersion
		}
	}
	if string(migrated) != string(data) {
		return os.WriteFile(path, migrated, 0644)
	}
	return nil
}

func detectVersion(data []byte) int {
	var h struct{ Version int `json:"version"` }
	if err := json.Unmarshal(data, &h); err != nil || h.Version == 0 {
		return 1
	}
	return h.Version
}

func ShowDeprecationWarnings() {
	home, _ := os.UserHomeDir()
	for _, p := range []string{filepath.Join(home, ".pi"), filepath.Join(home, ".pi-config.json")} {
		if _, err := os.Stat(p); err == nil {
			fmt.Fprintf(os.Stderr, "Warning: Found old config at %s. Consider migrating to ~/.rho/\n", p)
		}
	}
}

func init() {
	RegisterMigration(Migration{FromVersion: 1, ToVersion: 2, Description: "Add timestamps", Migrate: func(d []byte) ([]byte, error) { return d, nil }})
	RegisterMigration(Migration{FromVersion: 2, ToVersion: 3, Description: "Add model info", Migrate: func(d []byte) ([]byte, error) { return d, nil }})
}
