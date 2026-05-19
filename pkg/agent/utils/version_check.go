package agentutils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ReleaseInfo holds information about a release.
type ReleaseInfo struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
}

// UpdateChecker checks for newer versions.
type UpdateChecker struct {
	CurrentVersion string
	RepoOwner      string
	RepoName       string
	HTTPClient     *http.Client
}

// NewUpdateChecker creates an update checker.
func NewUpdateChecker(currentVersion, repoOwner, repoName string) *UpdateChecker {
	return &UpdateChecker{
		CurrentVersion: currentVersion,
		RepoOwner:      repoOwner,
		RepoName:       repoName,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// CheckLatest checks the latest release version from GitHub.
func (uc *UpdateChecker) CheckLatest() (*ReleaseInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", uc.RepoOwner, uc.RepoName)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", GetUserAgent())

	resp, err := uc.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("release API returned status %d", resp.StatusCode)
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("cannot parse release: %w", err)
	}

	return &release, nil
}

// IsNewerVersionAvailable compares the current version with the latest.
func (uc *UpdateChecker) IsNewerVersionAvailable(latestVersion string) bool {
	latest := strings.TrimPrefix(latestVersion, "v")
	current := strings.TrimPrefix(uc.CurrentVersion, "v")
	return CompareVersions(latest, current) > 0
}

// FormatUpdateNotification formats an update notification message.
func FormatUpdateNotification(currentVersion, latestVersion, releaseURL string) string {
	return fmt.Sprintf("Update available: %s → %s\n%s",
		currentVersion, latestVersion, releaseURL)
}

// CheckForUpdates performs a full update check and returns a notification if available.
func CheckForUpdates(currentVersion, repoOwner, repoName string) string {
	checker := NewUpdateChecker(currentVersion, repoOwner, repoName)
	release, err := checker.CheckLatest()
	if err != nil {
		return ""
	}
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	if checker.IsNewerVersionAvailable(latestVersion) {
		return FormatUpdateNotification(
			currentVersion,
			latestVersion,
			release.HTMLURL,
		)
	}
	return ""
}


