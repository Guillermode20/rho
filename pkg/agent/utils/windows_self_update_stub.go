//go:build !windows
// +build !windows

package agentutils

import "time"

// WindowsSelfUpdate is a stub for non-Windows platforms.
type WindowsSelfUpdate struct{}

func (wsu *WindowsSelfUpdate) IsMarkedForCleanup() bool { return false }

func (wsu *WindowsSelfUpdate) ReplaceRunning(newBinary []byte) error { return nil }

func CleanupWindowsSelfUpdateQuarantine() error { return nil }

func ScheduleUpdate(newBinaryPath string) error { return nil }

func WaitForUpdateCompletion(maxWait time.Duration) error { return nil }

func IsUpdatePending() bool { return false }
