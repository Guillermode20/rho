// Package agentutils provides utility functions for agent operations.
package agentutils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// SelfUpdater handles checking for and applying updates.
type SelfUpdater struct {
	CurrentVersion string
	UpdateURL      string
	CheckInterval  time.Duration
	lastCheck      time.Time
	UpdateInfo     *UpdateInfo
}

// UpdateInfo describes an available update.
type UpdateInfo struct {
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	ReleaseDate string `json:"release_date"`
	Description string `json:"description"`
	Checksum    string `json:"checksum"`
}

// NewSelfUpdater creates a new updater.
func NewSelfUpdater(currentVersion, updateURL string) *SelfUpdater {
	return &SelfUpdater{
		CurrentVersion: currentVersion,
		UpdateURL:      updateURL,
		CheckInterval:  24 * time.Hour,
	}
}

// CheckForUpdates checks for a new version.
func (u *SelfUpdater) CheckForUpdates() (*UpdateInfo, error) {
	u.lastCheck = time.Now()

	// In production, this would fetch from a releases API
	// For now, use a simulated check
	if u.UpdateURL == "" || strings.HasPrefix(u.UpdateURL, "https://") {
		return u.checkViaAPI()
	}

	return nil, nil
}

func (u *SelfUpdater) checkViaAPI() (*UpdateInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(u.UpdateURL)
	if err != nil {
		return nil, fmt.Errorf("update check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, nil
	}

	var release struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, nil
	}

	// Filter assets for current platform
	assetName := fmt.Sprintf("rho-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}

	var downloadURL string
	for _, a := range release.Assets {
		if strings.Contains(a.Name, assetName) || strings.Contains(a.Name, runtime.GOOS) {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}

	if release.TagName == "" || downloadURL == "" {
		return nil, nil
	}

	version := strings.TrimPrefix(release.TagName, "v")
	if compareVersions(version, u.CurrentVersion) <= 0 {
		return nil, nil
	}

	info := &UpdateInfo{
		Version:     version,
		DownloadURL: downloadURL,
		Description: release.Body,
	}
	u.UpdateInfo = info
	return info, nil
}

// ApplyUpdate downloads and applies an update.
func (u *SelfUpdater) ApplyUpdate(info *UpdateInfo) error {
	if info == nil {
		return fmt.Errorf("no update info available")
	}

	fmt.Printf("Downloading rho v%s...\n", info.Version)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(info.DownloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	// Download to temp file
	tmpDir, err := os.MkdirTemp("", "rho-update-*")
	if err != nil {
		return fmt.Errorf("cannot create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpPath := filepath.Join(tmpDir, "rho-update")
	if runtime.GOOS == "windows" {
		tmpPath += ".exe"
	}

	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("cannot create temp file: %w", err)
	}

	written, err := io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	if written == 0 {
		return fmt.Errorf("downloaded file is empty")
	}

	// Make executable
	os.Chmod(tmpPath, 0755)

	// Get current binary path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	// Replace binary
	if err := os.Rename(tmpPath, execPath); err != nil {
		// Fallback: copy via command
		return u.replaceViaCopy(tmpPath, execPath)
	}

	fmt.Printf("Updated to v%s! Please restart rho.\n", info.Version)
	return nil
}

func (u *SelfUpdater) replaceViaCopy(src, dst string) error {
	cmd := exec.Command("cp", src, dst)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}
	return nil
}

// NotifyIfUpdateAvailable returns a notification string if an update is available.
func (u *SelfUpdater) NotifyIfUpdateAvailable() string {
	if u.UpdateInfo == nil {
		return ""
	}
	return fmt.Sprintf("Update available: rho v%s (current: v%s). Run 'rho update' to upgrade.",
		u.UpdateInfo.Version, u.CurrentVersion)
}

// ShouldCheck returns true if enough time has passed since the last check.
func (u *SelfUpdater) ShouldCheck() bool {
	return time.Since(u.lastCheck) > u.CheckInterval
}

// compareVersions compares two semver strings. Returns -1, 0, or 1.
func compareVersions(a, b string) int {
	ap := strings.Split(strings.TrimPrefix(a, "v"), ".")
	bp := strings.Split(strings.TrimPrefix(b, "v"), ".")

	for i := 0; i < 3; i++ {
		ai, bi := 0, 0
		if i < len(ap) {
			fmt.Sscanf(ap[i], "%d", &ai)
		}
		if i < len(bp) {
			fmt.Sscanf(bp[i], "%d", &bi)
		}
		if ai > bi {
			return 1
		}
		if ai < bi {
			return -1
		}
	}
	return 0
}

var _ = io.Discard
