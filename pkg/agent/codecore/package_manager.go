package codecore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PackageSource string

const (
	SourceLocal  PackageSource = "local"
	SourceNPM    PackageSource = "npm"
	SourceGoPath PackageSource = "gopath"
	SourceGit    PackageSource = "git"
)

type PackageInfo struct {
	Name        string        `json:"name"`
	Version     string        `json:"version"`
	Source      PackageSource `json:"source"`
	Description string        `json:"description"`
	Enabled     bool          `json:"enabled"`
}

type PackageManager struct {
	ExtensionsDir string
}

func NewPackageManager(extDir string) *PackageManager {
	return &PackageManager{ExtensionsDir: extDir}
}

func (pm *PackageManager) ListInstalledPackages() ([]PackageInfo, error) {
	os.MkdirAll(pm.ExtensionsDir, 0755)
	entries, err := os.ReadDir(pm.ExtensionsDir)
	if err != nil {
		return nil, err
	}
	var pkgs []PackageInfo
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		pkg := PackageInfo{Name: e.Name(), Source: SourceLocal, Enabled: true}
		if data, err := os.ReadFile(filepath.Join(pm.ExtensionsDir, e.Name(), "extension.json")); err == nil {
			var m struct {
				Name    string `json:"name"`
				Version string `json:"version"`
				Desc    string `json:"description"`
			}
			if json.Unmarshal(data, &m) == nil {
				pkg.Version = m.Version
				pkg.Description = m.Desc
				if m.Name != "" {
					pkg.Name = m.Name
				}
			}
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}

func (pm *PackageManager) InstallPackage(name string, source PackageSource) error {
	dir := filepath.Join(pm.ExtensionsDir, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	m := map[string]interface{}{"name": name, "version": "0.1.0", "source": string(source)}
	d, _ := json.MarshalIndent(m, "", "  ")
	return os.WriteFile(filepath.Join(dir, "extension.json"), d, 0644)
}

func (pm *PackageManager) RemovePackage(name string) error {
	return os.RemoveAll(filepath.Join(pm.ExtensionsDir, name))
}

func (pm *PackageManager) EnablePackage(name string) error {
	d := filepath.Join(pm.ExtensionsDir, name, ".enabled")
	return os.WriteFile(d, []byte("enabled"), 0644)
}

func (pm *PackageManager) DisablePackage(name string) error {
	return os.Remove(filepath.Join(pm.ExtensionsDir, name, ".enabled"))
}

func ResolvePackagePath(name string, dirs []string) string {
	for _, d := range dirs {
		p := filepath.Join(d, name)
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p
		}
	}
	return ""
}

var _ = fmt.Sprintf
