package agentutils

import (
	"fmt"
	"runtime"
)

var (
	appName    = "rho"
	appVersion = "0.2.0"
)

// SetAppInfo sets the application name and version for user agent generation.
func SetAppInfo(name, version string) {
	appName = name
	appVersion = version
}

// GetUserAgent returns the user agent string for API requests.
func GetUserAgent() string {
	return fmt.Sprintf("%s/%s (%s; %s)", appName, appVersion, runtime.GOOS, runtime.GOARCH)
}

// GetUserAgentWithExtra returns the user agent with additional context.
func GetUserAgentWithExtra(extra string) string {
	ua := GetUserAgent()
	if extra != "" {
		ua = ua + "; " + extra
	}
	return ua
}

// AppName returns the current application name.
func AppName() string {
	return appName
}

// AppVersion returns the current application version.
func AppVersion() string {
	return appVersion
}

// PiUserAgent returns the user agent used for pi/rho API requests.
// This matches the pattern of various AI provider SDKs.
func PiUserAgent() string {
	return fmt.Sprintf("%s%s; %s", appName, appVersion, runtime.GOOS)
}
