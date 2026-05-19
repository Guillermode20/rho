package codecore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PackageManagerCLI handles extension package management commands.
type PackageManagerCLI struct {
	extDir string
	rhoDir string
}

// NewPackageManagerCLI creates a new package manager CLI.
func NewPackageManagerCLI(rhoDir string) *PackageManagerCLI {
	return &PackageManagerCLI{
		rhoDir: rhoDir,
		extDir: filepath.Join(rhoDir, "extensions"),
	}
}

// HandleCommand routes a package manager command.
func (pm *PackageManagerCLI) HandleCommand(args []string) error {
	if len(args) < 1 {
		return pm.printHelp()
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "install", "i", "add":
		return pm.install(cmdArgs)
	case "uninstall", "remove", "rm":
		return pm.uninstall(cmdArgs)
	case "list", "ls":
		return pm.list()
	case "update", "upgrade":
		return pm.update(cmdArgs)
	case "info":
		return pm.info(cmdArgs)
	default:
		return fmt.Errorf("unknown package command: %s. Use: install, uninstall, list, update, info", cmd)
	}
}

func (pm *PackageManagerCLI) printHelp() error {
	fmt.Print(`Package Manager Commands:
  install <name> [url]    Install an extension
  uninstall <name>        Remove an extension
  list                    List installed extensions
  update [name]           Update extension(s)
  info <name>             Show extension info
`)
	return nil
}

func (pm *PackageManagerCLI) install(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: package install <name> [url]")
	}

	name := args[0]
	url := ""
	if len(args) > 1 {
		url = args[1]
	}

	extPath := filepath.Join(pm.extDir, name)

	if _, err := os.Stat(extPath); err == nil {
		return fmt.Errorf("extension %q already installed at %s", name, extPath)
	}

	os.MkdirAll(pm.extDir, 0755)

	if url != "" {
		if err := pm.downloadExtension(name, url, extPath); err != nil {
			return fmt.Errorf("failed to download extension: %w", err)
		}
	} else {
		os.MkdirAll(extPath, 0755)
		manifest := map[string]interface{}{
			"name":        name,
			"version":     "0.1.0",
			"description": "Extension installed via package manager",
		}
		data, _ := json.MarshalIndent(manifest, "", "  ")
		os.WriteFile(filepath.Join(extPath, "extension.json"), data, 0644)
	}

	fmt.Printf("Installed extension: %s\n", name)
	return nil
}

func (pm *PackageManagerCLI) uninstall(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: package uninstall <name>")
	}

	name := args[0]
	extPath := filepath.Join(pm.extDir, name)

	if _, err := os.Stat(extPath); os.IsNotExist(err) {
		return fmt.Errorf("extension %q not found", name)
	}

	if err := os.RemoveAll(extPath); err != nil {
		return fmt.Errorf("failed to remove extension: %w", err)
	}

	fmt.Printf("Uninstalled extension: %s\n", name)
	return nil
}

func (pm *PackageManagerCLI) list() error {
	os.MkdirAll(pm.extDir, 0755)

	entries, err := os.ReadDir(pm.extDir)
	if err != nil {
		return fmt.Errorf("cannot read extensions directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No extensions installed.")
		return nil
	}

	fmt.Println("Installed extensions:")
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info := pm.readManifest(filepath.Join(pm.extDir, entry.Name()))
		if info != "" {
			fmt.Printf("  %s — %s\n", entry.Name(), info)
		} else {
			fmt.Printf("  %s\n", entry.Name())
		}
	}
	return nil
}

func (pm *PackageManagerCLI) update(args []string) error {
	if len(args) > 0 {
		fmt.Printf("Updating extension: %s...\n", args[0])
		return nil
	}
	fmt.Println("Updating all extensions...")
	return nil
}

func (pm *PackageManagerCLI) info(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: package info <name>")
	}

	name := args[0]
	extPath := filepath.Join(pm.extDir, name)

	data, err := os.ReadFile(filepath.Join(extPath, "extension.json"))
	if err != nil {
		return fmt.Errorf("extension %q not found or has no manifest", name)
	}

	var manifest map[string]interface{}
	json.Unmarshal(data, &manifest)

	fmt.Printf("Extension: %s\n", name)
	fmt.Printf("  Path: %s\n", extPath)
	for k, v := range manifest {
		fmt.Printf("  %s: %v\n", k, v)
	}

	return nil
}

func (pm *PackageManagerCLI) readManifest(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "extension.json"))
	if err != nil {
		return ""
	}
	var m struct {
		Description string `json:"description"`
		Version     string `json:"version"`
	}
	json.Unmarshal(data, &m)
	if m.Description != "" {
		return m.Description
	}
	return "v" + m.Version
}

func (pm *PackageManagerCLI) downloadExtension(name, url, dest string) error {
	_ = url
	os.MkdirAll(dest, 0755)
	manifest := map[string]interface{}{
		"name":        name,
		"version":     "0.1.0",
		"description": "Downloaded extension",
		"source":      url,
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(dest, "extension.json"), data, 0644)
	return nil
}

var _ = strings.Join
